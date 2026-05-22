/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package multicloudsubscription

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	multicloudsdk "github.com/oracle/oci-go-sdk/v65/multicloud"
	multicloudv1beta1 "github.com/oracle/oci-service-operator/api/multicloud/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	multicloudSubscriptionCompartmentIDAnnotation                 = "multicloud.oracle.com/compartment-id"
	multicloudSubscriptionDisplayNameAnnotation                   = "multicloud.oracle.com/display-name"
	multicloudSubscriptionSubscriptionIDAnnotation                = "multicloud.oracle.com/subscription-id"
	multicloudSubscriptionClassicSubscriptionIDAnnotation         = "multicloud.oracle.com/classic-subscription-id"
	multicloudSubscriptionPartnerCloudAccountIdentifierAnnotation = "multicloud.oracle.com/partner-cloud-account-identifier"
	multicloudSubscriptionServiceNameAnnotation                   = "multicloud.oracle.com/service-name"
	multicloudSubscriptionListPageLimit                           = 100
	multicloudSubscriptionDeleteUnsupportedMessage                = "OCI MulticloudSubscription delete is not supported by the multicloud SDK; no remote mutation was issued"
	multicloudSubscriptionCreateUnsupportedMessage                = "OCI SDK exposes ListMulticloudSubscriptions only and cannot create a missing MulticloudSubscription"
)

type listMulticloudSubscriptionsFunc func(context.Context, multicloudsdk.ListMulticloudSubscriptionsRequest) (multicloudsdk.ListMulticloudSubscriptionsResponse, error)

type multicloudSubscriptionRuntimeClient struct {
	delegate MulticloudSubscriptionServiceClient
	list     listMulticloudSubscriptionsFunc
	provider common.ConfigurationProvider
	log      loggerutil.OSOKLogger
}

type multicloudSubscriptionIdentity struct {
	compartmentID                 string
	displayName                   string
	subscriptionID                string
	classicSubscriptionID         string
	partnerCloudAccountIdentifier string
	serviceName                   string
}

func init() {
	registerMulticloudSubscriptionRuntimeHooksMutator(applyMulticloudSubscriptionRuntimeHooks)
}

func applyMulticloudSubscriptionRuntimeHooks(
	manager *MulticloudSubscriptionServiceManager,
	hooks *MulticloudSubscriptionRuntimeHooks,
) {
	list := hooks.List.Call
	hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(delegate MulticloudSubscriptionServiceClient) MulticloudSubscriptionServiceClient {
		return &multicloudSubscriptionRuntimeClient{
			delegate: delegate,
			list:     list,
			provider: manager.Provider,
			log:      manager.Log,
		}
	})
}

func (c *multicloudSubscriptionRuntimeClient) CreateOrUpdate(
	ctx context.Context,
	resource *multicloudv1beta1.MulticloudSubscription,
	_ ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	identity, err := c.resolveIdentity(resource)
	if err != nil {
		return c.fail(resource, err)
	}

	summary, requestID, err := c.findSubscription(ctx, identity)
	if requestID != "" {
		servicemanager.SetOpcRequestID(&resource.Status.OsokStatus, requestID)
	}
	if err != nil {
		return c.fail(resource, err)
	}

	c.projectStatus(resource, summary, requestID)
	return servicemanager.OSOKResponse{IsSuccessful: true}, nil
}

func (c *multicloudSubscriptionRuntimeClient) Delete(_ context.Context, resource *multicloudv1beta1.MulticloudSubscription) (bool, error) {
	c.markDeleted(resource, multicloudSubscriptionDeleteUnsupportedMessage)
	return true, nil
}

func (c *multicloudSubscriptionRuntimeClient) resolveIdentity(
	resource *multicloudv1beta1.MulticloudSubscription,
) (multicloudSubscriptionIdentity, error) {
	annotations := resource.GetAnnotations()
	subscriptionID, err := resolvedReadOnlySelector(
		"subscriptionId",
		resource.Status.SubscriptionId,
		annotationValue(annotations, multicloudSubscriptionSubscriptionIDAnnotation),
		multicloudSubscriptionSubscriptionIDAnnotation,
	)
	if err != nil {
		return multicloudSubscriptionIdentity{}, err
	}
	classicSubscriptionID, err := resolvedReadOnlySelector(
		"classicSubscriptionId",
		resource.Status.ClassicSubscriptionId,
		annotationValue(annotations, multicloudSubscriptionClassicSubscriptionIDAnnotation),
		multicloudSubscriptionClassicSubscriptionIDAnnotation,
	)
	if err != nil {
		return multicloudSubscriptionIdentity{}, err
	}
	partnerCloudAccountIdentifier, err := resolvedReadOnlySelector(
		"partnerCloudAccountIdentifier",
		resource.Status.PartnerCloudAccountIdentifier,
		annotationValue(annotations, multicloudSubscriptionPartnerCloudAccountIdentifierAnnotation),
		multicloudSubscriptionPartnerCloudAccountIdentifierAnnotation,
	)
	if err != nil {
		return multicloudSubscriptionIdentity{}, err
	}
	serviceName, err := resolvedReadOnlySelector(
		"serviceName",
		resource.Status.ServiceName,
		annotationValue(annotations, multicloudSubscriptionServiceNameAnnotation),
		multicloudSubscriptionServiceNameAnnotation,
	)
	if err != nil {
		return multicloudSubscriptionIdentity{}, err
	}

	compartmentID := annotationValue(annotations, multicloudSubscriptionCompartmentIDAnnotation)
	if compartmentID == "" {
		compartmentID, err = c.tenancyOCID()
		if err != nil {
			return multicloudSubscriptionIdentity{}, err
		}
	}

	displayName := annotationValue(annotations, multicloudSubscriptionDisplayNameAnnotation)
	if displayName == "" {
		displayName = strings.TrimSpace(resource.Name)
	}

	return multicloudSubscriptionIdentity{
		compartmentID:                 compartmentID,
		displayName:                   displayName,
		subscriptionID:                subscriptionID,
		classicSubscriptionID:         classicSubscriptionID,
		partnerCloudAccountIdentifier: partnerCloudAccountIdentifier,
		serviceName:                   serviceName,
	}, nil
}

func (c *multicloudSubscriptionRuntimeClient) tenancyOCID() (string, error) {
	if c.provider == nil {
		return "", fmt.Errorf("MulticloudSubscription requires %q annotation because the OCI provider is unavailable", multicloudSubscriptionCompartmentIDAnnotation)
	}
	tenancyID, err := c.provider.TenancyOCID()
	if err != nil {
		return "", fmt.Errorf("resolve MulticloudSubscription tenancy compartment: %w", err)
	}
	if tenancyID = strings.TrimSpace(tenancyID); tenancyID == "" {
		return "", fmt.Errorf("MulticloudSubscription requires %q annotation because the OCI provider returned an empty tenancy OCID", multicloudSubscriptionCompartmentIDAnnotation)
	}
	return tenancyID, nil
}

func (c *multicloudSubscriptionRuntimeClient) findSubscription(
	ctx context.Context,
	identity multicloudSubscriptionIdentity,
) (multicloudsdk.MulticloudSubscriptionSummary, string, error) {
	if c.list == nil {
		return multicloudsdk.MulticloudSubscriptionSummary{}, "", fmt.Errorf("MulticloudSubscription runtime has no ListMulticloudSubscriptions client")
	}

	var (
		matches       []multicloudsdk.MulticloudSubscriptionSummary
		page          string
		lastRequestID string
	)
	for {
		response, err := c.list(ctx, identity.listRequest(page))
		if requestID := stringValue(response.OpcRequestId); requestID != "" {
			lastRequestID = requestID
		}
		if err != nil {
			return multicloudsdk.MulticloudSubscriptionSummary{}, lastRequestID, fmt.Errorf("list MulticloudSubscriptions: %w", err)
		}
		for _, item := range response.Items {
			if identity.matches(item) {
				matches = append(matches, item)
			}
		}
		page = stringValue(response.OpcNextPage)
		if page == "" {
			break
		}
	}

	switch len(matches) {
	case 0:
		return multicloudsdk.MulticloudSubscriptionSummary{}, lastRequestID, fmt.Errorf(
			"MulticloudSubscription %s was not found in compartment %q; %s",
			identity.selectorDescription(),
			identity.compartmentID,
			multicloudSubscriptionCreateUnsupportedMessage,
		)
	case 1:
		return matches[0], lastRequestID, nil
	default:
		return multicloudsdk.MulticloudSubscriptionSummary{}, lastRequestID, fmt.Errorf(
			"MulticloudSubscription %s matched %d OCI subscriptions in compartment %q; add a more specific metadata annotation",
			identity.selectorDescription(),
			len(matches),
			identity.compartmentID,
		)
	}
}

func (i multicloudSubscriptionIdentity) listRequest(page string) multicloudsdk.ListMulticloudSubscriptionsRequest {
	request := multicloudsdk.ListMulticloudSubscriptionsRequest{
		CompartmentId: common.String(i.compartmentID),
		Limit:         common.Int(multicloudSubscriptionListPageLimit),
	}
	if i.shouldUseDisplayNameFilter() {
		request.DisplayName = common.String(i.displayName)
	}
	if page != "" {
		request.Page = common.String(page)
	}
	return request
}

func (i multicloudSubscriptionIdentity) shouldUseDisplayNameFilter() bool {
	return i.displayName != "" &&
		i.subscriptionID == "" &&
		i.classicSubscriptionID == "" &&
		i.partnerCloudAccountIdentifier == "" &&
		i.serviceName == ""
}

func (i multicloudSubscriptionIdentity) matches(item multicloudsdk.MulticloudSubscriptionSummary) bool {
	compared := false
	if i.subscriptionID != "" {
		compared = true
		if stringValue(item.SubscriptionId) != i.subscriptionID {
			return false
		}
	}
	if i.classicSubscriptionID != "" {
		compared = true
		if stringValue(item.ClassicSubscriptionId) != i.classicSubscriptionID {
			return false
		}
	}
	if i.partnerCloudAccountIdentifier != "" {
		compared = true
		if stringValue(item.PartnerCloudAccountIdentifier) != i.partnerCloudAccountIdentifier {
			return false
		}
	}
	if i.serviceName != "" {
		compared = true
		if string(item.ServiceName) != i.serviceName {
			return false
		}
	}
	return compared || i.shouldUseDisplayNameFilter()
}

func (i multicloudSubscriptionIdentity) selectorDescription() string {
	var parts []string
	for _, part := range []struct {
		name  string
		value string
	}{
		{name: "subscriptionId", value: i.subscriptionID},
		{name: "classicSubscriptionId", value: i.classicSubscriptionID},
		{name: "partnerCloudAccountIdentifier", value: i.partnerCloudAccountIdentifier},
		{name: "serviceName", value: i.serviceName},
	} {
		if part.value != "" {
			parts = append(parts, fmt.Sprintf("%s=%q", part.name, part.value))
		}
	}
	if len(parts) == 0 && i.shouldUseDisplayNameFilter() {
		parts = append(parts, fmt.Sprintf("displayName=%q", i.displayName))
	}
	if len(parts) == 0 {
		return "without a selector"
	}
	return strings.Join(parts, ", ")
}

func (c *multicloudSubscriptionRuntimeClient) projectStatus(
	resource *multicloudv1beta1.MulticloudSubscription,
	summary multicloudsdk.MulticloudSubscriptionSummary,
	requestID string,
) {
	resource.Status.ClassicSubscriptionId = stringValue(summary.ClassicSubscriptionId)
	resource.Status.PartnerCloudAccountIdentifier = stringValue(summary.PartnerCloudAccountIdentifier)
	resource.Status.TimeCreated = sdkTimeString(summary.TimeCreated)
	resource.Status.SubscriptionId = stringValue(summary.SubscriptionId)
	resource.Status.ServiceName = string(summary.ServiceName)
	resource.Status.TimeLinkedDate = sdkTimeString(summary.TimeLinkedDate)
	resource.Status.PaymentPlan = stringValue(summary.PaymentPlan)
	resource.Status.ActiveCommitment = stringValue(summary.ActiveCommitment)
	resource.Status.TimeEndDate = sdkTimeString(summary.TimeEndDate)
	resource.Status.LifecycleState = string(summary.LifecycleState)
	resource.Status.CspAdditionalProperties = cloneStringMap(summary.CspAdditionalProperties)
	resource.Status.TimeUpdated = sdkTimeString(summary.TimeUpdated)
	resource.Status.FreeformTags = cloneStringMap(summary.FreeformTags)
	resource.Status.DefinedTags = mapValueTags(summary.DefinedTags)
	resource.Status.SystemTags = mapValueTags(summary.SystemTags)

	status := &resource.Status.OsokStatus
	servicemanager.SetOpcRequestID(status, requestID)
	if subscriptionID := resource.Status.SubscriptionId; strings.HasPrefix(subscriptionID, "ocid1.") && len(subscriptionID) <= 255 {
		status.Ocid = shared.OCID(subscriptionID)
	} else {
		status.Ocid = ""
	}
	status.Async.Current = nil

	now := metav1.Now()
	if status.CreatedAt == nil {
		status.CreatedAt = &now
	}
	status.UpdatedAt = &now
	status.Reason = string(shared.Active)
	status.Message = multicloudSubscriptionObservedMessage(resource.Status.LifecycleState)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Active, corev1.ConditionTrue, "", status.Message, c.log)
}

func (c *multicloudSubscriptionRuntimeClient) fail(
	resource *multicloudv1beta1.MulticloudSubscription,
	err error,
) (servicemanager.OSOKResponse, error) {
	status := &resource.Status.OsokStatus
	servicemanager.RecordErrorOpcRequestID(status, err)
	now := metav1.Now()
	status.UpdatedAt = &now
	status.Reason = string(shared.Failed)
	status.Message = err.Error()
	*status = util.UpdateOSOKStatusCondition(*status, shared.Failed, corev1.ConditionFalse, "", err.Error(), c.log)
	return servicemanager.OSOKResponse{IsSuccessful: false}, err
}

func (c *multicloudSubscriptionRuntimeClient) markDeleted(resource *multicloudv1beta1.MulticloudSubscription, message string) {
	status := &resource.Status.OsokStatus
	now := metav1.Now()
	status.DeletedAt = &now
	status.UpdatedAt = &now
	status.Message = message
	status.Reason = string(shared.Terminating)
	status.Async.Current = nil
	*status = util.UpdateOSOKStatusCondition(*status, shared.Terminating, corev1.ConditionTrue, "", message, c.log)
}

func resolvedReadOnlySelector(fieldName string, recorded string, annotated string, annotationName string) (string, error) {
	recorded = strings.TrimSpace(recorded)
	annotated = strings.TrimSpace(annotated)
	if recorded != "" && annotated != "" && recorded != annotated {
		return "", fmt.Errorf(
			"MulticloudSubscription read-only selector annotation %q changed %s from %q to %q; create a replacement resource instead",
			annotationName,
			fieldName,
			recorded,
			annotated,
		)
	}
	if recorded != "" {
		return recorded, nil
	}
	return annotated, nil
}

func annotationValue(annotations map[string]string, key string) string {
	if annotations == nil {
		return ""
	}
	return strings.TrimSpace(annotations[key])
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func sdkTimeString(value *common.SDKTime) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func mapValueTags(values map[string]map[string]any) map[string]shared.MapValue {
	if len(values) == 0 {
		return nil
	}
	converted := make(map[string]shared.MapValue, len(values))
	for namespace, tags := range values {
		converted[namespace] = make(shared.MapValue, len(tags))
		for key, value := range tags {
			if value == nil {
				continue
			}
			converted[namespace][key] = fmt.Sprint(value)
		}
	}
	return converted
}

func multicloudSubscriptionObservedMessage(lifecycleState string) string {
	if lifecycleState == "" {
		return "OCI MulticloudSubscription observed"
	}
	return fmt.Sprintf("OCI MulticloudSubscription observed in lifecycle state %s", lifecycleState)
}
