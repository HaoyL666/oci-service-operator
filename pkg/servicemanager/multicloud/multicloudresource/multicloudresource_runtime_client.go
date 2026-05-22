/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package multicloudresource

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
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	"github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	multicloudResourceSubscriptionServiceNameAnnotation = "multicloud.oracle.com/subscription-service-name"
	multicloudResourceSubscriptionIDAnnotation          = "multicloud.oracle.com/subscription-id"
	multicloudResourceIDAnnotation                      = "multicloud.oracle.com/resource-id"
	multicloudResourceAnchorIDAnnotation                = "multicloud.oracle.com/resource-anchor-id"
	multicloudResourceCompartmentIDAnnotation           = "multicloud.oracle.com/compartment-id"
	multicloudResourceExternalLocationAnnotation        = "multicloud.oracle.com/external-location"
	multicloudResourceDisplayNameAnnotation             = "multicloud.oracle.com/resource-display-name"
	multicloudResourceTypeAnnotation                    = "multicloud.oracle.com/resource-type"
	multicloudResourceCSPResourceIDAnnotation           = "multicloud.oracle.com/csp-resource-id"
	multicloudResourceNetworkAnchorIDAnnotation         = "multicloud.oracle.com/network-anchor-id"

	multicloudResourceLegacySubscriptionServiceNameAnnotation = "multicloud.oracle.com/subscriptionServiceName"
	multicloudResourceLegacySubscriptionIDAnnotation          = "multicloud.oracle.com/subscriptionId"
	multicloudResourceLegacyIDAnnotation                      = "multicloud.oracle.com/resourceId"

	multicloudResourceListLimit = 100
)

type multicloudResourceOCIClient interface {
	ListMulticloudResources(context.Context, multicloudsdk.ListMulticloudResourcesRequest) (multicloudsdk.ListMulticloudResourcesResponse, error)
}

type multicloudResourceRuntimeClient struct {
	list    func(context.Context, multicloudsdk.ListMulticloudResourcesRequest) (multicloudsdk.ListMulticloudResourcesResponse, error)
	initErr error
	log     loggerutil.OSOKLogger
}

type multicloudResourceSelector struct {
	subscriptionServiceName string
	subscriptionID          string
	resourceID              string
	resourceAnchorID        string
	compartmentID           string
	externalLocation        string
	resourceDisplayName     string
	resourceType            string
	cspResourceID           string
	networkAnchorID         string
}

func init() {
	registerMulticloudResourceRuntimeHooksMutator(func(manager *MulticloudResourceServiceManager, hooks *MulticloudResourceRuntimeHooks) {
		applyMulticloudResourceRuntimeHooks(manager, hooks)
	})
}

func applyMulticloudResourceRuntimeHooks(manager *MulticloudResourceServiceManager, hooks *MulticloudResourceRuntimeHooks) {
	if hooks == nil {
		return
	}

	hooks.Semantics = multicloudResourceRuntimeSemantics()
	hooks.List.Fields = multicloudResourceListFields()
	hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(delegate MulticloudResourceServiceClient) MulticloudResourceServiceClient {
		var log loggerutil.OSOKLogger
		if manager != nil {
			log = manager.Log
		}
		return &multicloudResourceRuntimeClient{
			list:    hooks.List.Call,
			initErr: multicloudResourceGeneratedDelegateInitError(delegate),
			log:     log,
		}
	})
}

func multicloudResourceGeneratedDelegateInitError(delegate MulticloudResourceServiceClient) error {
	if delegate == nil {
		return nil
	}

	var resource *multicloudv1beta1.MulticloudResource
	_, err := delegate.Delete(context.Background(), resource)
	if isMulticloudResourceGeneratedInitError(err) {
		return err
	}
	return nil
}

func isMulticloudResourceGeneratedInitError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "initialize MulticloudResource OCI client")
}

func newMulticloudResourceServiceClientWithOCIClient(
	log loggerutil.OSOKLogger,
	client multicloudResourceOCIClient,
) MulticloudResourceServiceClient {
	hooks := newMulticloudResourceRuntimeHooksWithOCIClient(client)
	applyMulticloudResourceRuntimeHooks(&MulticloudResourceServiceManager{Log: log}, &hooks)
	delegate := defaultMulticloudResourceServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*multicloudv1beta1.MulticloudResource](
			buildMulticloudResourceGeneratedRuntimeConfig(&MulticloudResourceServiceManager{Log: log}, hooks),
		),
	}
	return wrapMulticloudResourceGeneratedClient(hooks, delegate)
}

func newMulticloudResourceRuntimeHooksWithOCIClient(client multicloudResourceOCIClient) MulticloudResourceRuntimeHooks {
	return MulticloudResourceRuntimeHooks{
		Identity:        generatedruntime.IdentityHooks[*multicloudv1beta1.MulticloudResource]{},
		Read:            generatedruntime.ReadHooks{},
		TrackedRecreate: generatedruntime.TrackedRecreateHooks[*multicloudv1beta1.MulticloudResource]{},
		StatusHooks:     generatedruntime.StatusHooks[*multicloudv1beta1.MulticloudResource]{},
		ParityHooks:     generatedruntime.ParityHooks[*multicloudv1beta1.MulticloudResource]{},
		Async:           generatedruntime.AsyncHooks[*multicloudv1beta1.MulticloudResource]{},
		DeleteHooks:     generatedruntime.DeleteHooks[*multicloudv1beta1.MulticloudResource]{},
		List: runtimeOperationHooks[multicloudsdk.ListMulticloudResourcesRequest, multicloudsdk.ListMulticloudResourcesResponse]{
			Fields: multicloudResourceListFields(),
			Call: func(ctx context.Context, request multicloudsdk.ListMulticloudResourcesRequest) (multicloudsdk.ListMulticloudResourcesResponse, error) {
				if client == nil {
					return multicloudsdk.ListMulticloudResourcesResponse{}, fmt.Errorf("multicloudresource OCI client is nil")
				}
				return client.ListMulticloudResources(ctx, request)
			},
		},
		WrapGeneratedClient: []func(MulticloudResourceServiceClient) MulticloudResourceServiceClient{},
	}
}

func multicloudResourceRuntimeSemantics() *generatedruntime.Semantics {
	return &generatedruntime.Semantics{
		FormalService:     "multicloud",
		FormalSlug:        "multicloudresource",
		StatusProjection:  "required",
		SecretSideEffects: "none",
		FinalizerPolicy:   "release-without-oci-delete",
		Async: &generatedruntime.AsyncSemantics{
			Strategy:             "lifecycle",
			Runtime:              "handwritten",
			FormalClassification: "lifecycle",
		},
		Lifecycle: generatedruntime.LifecycleSemantics{
			ActiveStates: []string{string(multicloudsdk.MulticloudResourceSummaryLifecycleStateActive)},
		},
		Delete: generatedruntime.DeleteSemantics{
			Policy: "not-supported",
		},
		List: &generatedruntime.ListSemantics{
			ResponseItemsField: "Items",
			MatchFields: []string{
				"resourceId",
				"resourceDisplayName",
				"resourceType",
				"compartmentId",
				"networkAnchorId",
				"cspResourceId",
			},
		},
		Unsupported: []generatedruntime.UnsupportedSemantic{
			{
				Category:      "create-update-delete",
				StopCondition: "vendored multicloud MulticloudResources client exposes only ListMulticloudResources",
			},
		},
	}
}

func multicloudResourceListFields() []generatedruntime.RequestField {
	return []generatedruntime.RequestField{
		{FieldName: "SubscriptionServiceName", RequestName: "subscriptionServiceName", Contribution: "query"},
		{FieldName: "SubscriptionId", RequestName: "subscriptionId", Contribution: "query"},
		{FieldName: "ResourceAnchorId", RequestName: "resourceAnchorId", Contribution: "query"},
		{FieldName: "CompartmentId", RequestName: "compartmentId", Contribution: "query"},
		{FieldName: "ExternalLocation", RequestName: "externalLocation", Contribution: "query"},
		{FieldName: "Limit", RequestName: "limit", Contribution: "query"},
		{FieldName: "Page", RequestName: "page", Contribution: "query"},
	}
}

func (c *multicloudResourceRuntimeClient) CreateOrUpdate(
	ctx context.Context,
	resource *multicloudv1beta1.MulticloudResource,
	_ ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	if c == nil {
		err := fmt.Errorf("multicloudresource runtime client is not configured")
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	if c.initErr != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, markMulticloudResourceFailed(resource, c.initErr, c.log)
	}
	if c.list == nil {
		err := fmt.Errorf("multicloudresource runtime client is not configured")
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	if resource == nil {
		err := fmt.Errorf("multicloudresource resource is nil")
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}

	selector, err := resolveMulticloudResourceSelector(resource)
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, markMulticloudResourceFailed(resource, err, c.log)
	}

	items, requestID, err := c.listMulticloudResourcePages(ctx, selector)
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, markMulticloudResourceFailed(resource, err, c.log)
	}
	servicemanager.SetOpcRequestID(&resource.Status.OsokStatus, requestID)

	item, err := selectMulticloudResourceSummary(selector, items)
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, markMulticloudResourceFailed(resource, err, c.log)
	}

	return projectMulticloudResourceStatus(resource, item, c.log), nil
}

func (c *multicloudResourceRuntimeClient) Delete(ctx context.Context, resource *multicloudv1beta1.MulticloudResource) (bool, error) {
	if resource == nil {
		return false, fmt.Errorf("multicloudresource resource is nil")
	}

	markMulticloudResourceDeleted(resource, "OCI MulticloudResource is list-only; no OCI delete operation is exposed", c.log)
	return true, nil
}

func (c *multicloudResourceRuntimeClient) listMulticloudResourcePages(
	ctx context.Context,
	selector multicloudResourceSelector,
) ([]multicloudsdk.MulticloudResourceSummary, string, error) {
	request, err := buildMulticloudResourceListRequest(selector)
	if err != nil {
		return nil, "", err
	}

	var items []multicloudsdk.MulticloudResourceSummary
	var lastRequestID string
	seenPages := map[string]struct{}{}
	for {
		response, err := c.list(ctx, request)
		if err != nil {
			return nil, "", err
		}
		if requestID := stringValue(response.OpcRequestId); requestID != "" {
			lastRequestID = requestID
		}
		items = append(items, response.Items...)

		nextPage := stringValue(response.OpcNextPage)
		if nextPage == "" {
			return items, lastRequestID, nil
		}
		if _, seen := seenPages[nextPage]; seen {
			return nil, lastRequestID, fmt.Errorf("multicloudresource list pagination repeated page token %q", nextPage)
		}
		seenPages[nextPage] = struct{}{}
		request.Page = common.String(nextPage)
	}
}

func buildMulticloudResourceListRequest(selector multicloudResourceSelector) (multicloudsdk.ListMulticloudResourcesRequest, error) {
	serviceName, ok := multicloudsdk.GetMappingListMulticloudResourcesSubscriptionServiceNameEnum(selector.subscriptionServiceName)
	if !ok {
		return multicloudsdk.ListMulticloudResourcesRequest{}, fmt.Errorf(
			"MulticloudResource metadata annotation %q has unsupported value %q; supported values: %s",
			multicloudResourceSubscriptionServiceNameAnnotation,
			selector.subscriptionServiceName,
			strings.Join(multicloudsdk.GetListMulticloudResourcesSubscriptionServiceNameEnumStringValues(), ", "),
		)
	}

	request := multicloudsdk.ListMulticloudResourcesRequest{
		SubscriptionServiceName: serviceName,
		SubscriptionId:          common.String(selector.subscriptionID),
		Limit:                   common.Int(multicloudResourceListLimit),
	}
	if selector.resourceAnchorID != "" {
		request.ResourceAnchorId = common.String(selector.resourceAnchorID)
	}
	if selector.compartmentID != "" {
		request.CompartmentId = common.String(selector.compartmentID)
	}
	if selector.externalLocation != "" {
		request.ExternalLocation = common.String(selector.externalLocation)
	}
	return request, nil
}

func resolveMulticloudResourceSelector(resource *multicloudv1beta1.MulticloudResource) (multicloudResourceSelector, error) {
	annotations := resource.GetAnnotations()
	selector := multicloudResourceSelector{
		subscriptionServiceName: annotationValue(annotations,
			multicloudResourceSubscriptionServiceNameAnnotation,
			multicloudResourceLegacySubscriptionServiceNameAnnotation,
		),
		subscriptionID: annotationValue(annotations,
			multicloudResourceSubscriptionIDAnnotation,
			multicloudResourceLegacySubscriptionIDAnnotation,
		),
		resourceID: annotationValue(annotations,
			multicloudResourceIDAnnotation,
			multicloudResourceLegacyIDAnnotation,
		),
		resourceAnchorID:    annotationValue(annotations, multicloudResourceAnchorIDAnnotation),
		compartmentID:       firstNonEmpty(annotationValue(annotations, multicloudResourceCompartmentIDAnnotation), resource.Status.CompartmentId),
		externalLocation:    annotationValue(annotations, multicloudResourceExternalLocationAnnotation),
		resourceDisplayName: firstNonEmpty(annotationValue(annotations, multicloudResourceDisplayNameAnnotation), resource.Status.ResourceDisplayName),
		resourceType:        firstNonEmpty(annotationValue(annotations, multicloudResourceTypeAnnotation), resource.Status.ResourceType),
		cspResourceID:       firstNonEmpty(annotationValue(annotations, multicloudResourceCSPResourceIDAnnotation), resource.Status.CspResourceId),
		networkAnchorID:     firstNonEmpty(annotationValue(annotations, multicloudResourceNetworkAnchorIDAnnotation), resource.Status.NetworkAnchorId),
	}

	recordedResourceID := firstNonEmpty(resource.Status.ResourceId, string(resource.Status.OsokStatus.Ocid))
	if recordedResourceID != "" && selector.resourceID != "" && recordedResourceID != selector.resourceID {
		return multicloudResourceSelector{}, fmt.Errorf(
			"MulticloudResource create-only metadata annotation %q changed from %q to %q; create a replacement resource instead",
			multicloudResourceIDAnnotation,
			recordedResourceID,
			selector.resourceID,
		)
	}
	selector.resourceID = firstNonEmpty(recordedResourceID, selector.resourceID)

	var missing []string
	if selector.subscriptionServiceName == "" {
		missing = append(missing, multicloudResourceSubscriptionServiceNameAnnotation)
	}
	if selector.subscriptionID == "" {
		missing = append(missing, multicloudResourceSubscriptionIDAnnotation)
	}
	if len(missing) > 0 {
		return multicloudResourceSelector{}, fmt.Errorf("MulticloudResource requires metadata annotations: %s", strings.Join(missing, ", "))
	}

	return selector, nil
}

func selectMulticloudResourceSummary(
	selector multicloudResourceSelector,
	items []multicloudsdk.MulticloudResourceSummary,
) (multicloudsdk.MulticloudResourceSummary, error) {
	var matches []multicloudsdk.MulticloudResourceSummary
	for _, item := range items {
		if multicloudResourceSummaryMatches(selector, item) {
			matches = append(matches, item)
		}
	}

	switch {
	case len(matches) == 1:
		return matches[0], nil
	case len(matches) > 1:
		return multicloudsdk.MulticloudResourceSummary{}, fmt.Errorf("MulticloudResource list returned multiple matching resources")
	case len(items) == 0:
		return multicloudsdk.MulticloudResourceSummary{}, fmt.Errorf("MulticloudResource list returned no resources")
	case !selector.hasMatchCriteria() && len(items) == 1:
		return items[0], nil
	case !selector.hasMatchCriteria():
		return multicloudsdk.MulticloudResourceSummary{}, fmt.Errorf("MulticloudResource requires %q or an additional match annotation because list returned %d resources", multicloudResourceIDAnnotation, len(items))
	default:
		return multicloudsdk.MulticloudResourceSummary{}, fmt.Errorf("MulticloudResource list returned no resource matching the requested identity")
	}
}

func multicloudResourceSummaryMatches(selector multicloudResourceSelector, item multicloudsdk.MulticloudResourceSummary) bool {
	if selector.resourceID != "" {
		return stringValue(item.ResourceId) == selector.resourceID
	}

	compared := false
	for _, criterion := range []struct {
		want string
		got  string
	}{
		{selector.resourceDisplayName, stringValue(item.ResourceDisplayName)},
		{selector.resourceType, stringValue(item.ResourceType)},
		{selector.compartmentID, stringValue(item.CompartmentId)},
		{selector.cspResourceID, stringValue(item.CspResourceId)},
		{selector.networkAnchorID, stringValue(item.NetworkAnchorId)},
	} {
		if criterion.want == "" {
			continue
		}
		compared = true
		if criterion.want != criterion.got {
			return false
		}
	}
	return compared
}

func (s multicloudResourceSelector) hasMatchCriteria() bool {
	return s.resourceID != "" ||
		s.resourceDisplayName != "" ||
		s.resourceType != "" ||
		s.compartmentID != "" ||
		s.cspResourceID != "" ||
		s.networkAnchorID != ""
}

func projectMulticloudResourceStatus(
	resource *multicloudv1beta1.MulticloudResource,
	item multicloudsdk.MulticloudResourceSummary,
	log loggerutil.OSOKLogger,
) servicemanager.OSOKResponse {
	resource.Status.ResourceId = stringValue(item.ResourceId)
	resource.Status.TimeCreated = sdkTimeString(item.TimeCreated)
	resource.Status.ResourceDisplayName = stringValue(item.ResourceDisplayName)
	resource.Status.ResourceType = stringValue(item.ResourceType)
	resource.Status.CompartmentName = stringValue(item.CompartmentName)
	resource.Status.CompartmentId = stringValue(item.CompartmentId)
	resource.Status.VcnName = stringValue(item.VcnName)
	resource.Status.VcnId = stringValue(item.VcnId)
	resource.Status.NetworkAnchorName = stringValue(item.NetworkAnchorName)
	resource.Status.NetworkAnchorId = stringValue(item.NetworkAnchorId)
	resource.Status.CspResourceId = stringValue(item.CspResourceId)
	resource.Status.CspAdditionalProperties = copyStringMap(item.CspAdditionalProperties)
	resource.Status.TimeUpdated = sdkTimeString(item.TimeUpdated)
	resource.Status.LifecycleState = string(item.LifecycleState)
	resource.Status.FreeformTags = copyStringMap(item.FreeformTags)
	resource.Status.DefinedTags = copyMapValue(item.DefinedTags)
	resource.Status.SystemTags = copyMapValue(item.SystemTags)

	status := &resource.Status.OsokStatus
	now := metav1.Now()
	if resource.Status.ResourceId != "" {
		status.Ocid = shared.OCID(resource.Status.ResourceId)
		if status.CreatedAt == nil {
			if item.TimeCreated != nil {
				createdAt := metav1.NewTime(item.TimeCreated.Time)
				status.CreatedAt = &createdAt
			} else {
				status.CreatedAt = &now
			}
		}
	}
	servicemanager.ClearAsyncOperation(status)
	status.UpdatedAt = &now

	condition := shared.Active
	status.Message = fmt.Sprintf("OCI MulticloudResource %q is active", resource.Status.ResourceId)
	if item.LifecycleState != "" && item.LifecycleState != multicloudsdk.MulticloudResourceSummaryLifecycleStateActive {
		condition = shared.Failed
		status.Message = fmt.Sprintf("OCI MulticloudResource %q is %s", resource.Status.ResourceId, item.LifecycleState)
	}
	status.Reason = string(condition)
	*status = util.UpdateOSOKStatusCondition(*status, condition, conditionStatusForMulticloudResource(condition), "", status.Message, log)

	return servicemanager.OSOKResponse{IsSuccessful: condition != shared.Failed}
}

func markMulticloudResourceFailed(
	resource *multicloudv1beta1.MulticloudResource,
	err error,
	log loggerutil.OSOKLogger,
) error {
	if resource == nil {
		return err
	}
	status := &resource.Status.OsokStatus
	servicemanager.RecordErrorOpcRequestID(status, err)
	servicemanager.ClearAsyncOperation(status)
	now := metav1.Now()
	status.Message = err.Error()
	status.Reason = string(shared.Failed)
	status.UpdatedAt = &now
	*status = util.UpdateOSOKStatusCondition(*status, shared.Failed, corev1.ConditionFalse, "", err.Error(), log)
	return err
}

func markMulticloudResourceDeleted(
	resource *multicloudv1beta1.MulticloudResource,
	message string,
	log loggerutil.OSOKLogger,
) {
	status := &resource.Status.OsokStatus
	now := metav1.Now()
	status.DeletedAt = &now
	status.UpdatedAt = &now
	status.Message = message
	status.Reason = string(shared.Terminating)
	servicemanager.ClearAsyncOperation(status)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Terminating, corev1.ConditionTrue, "", message, log)
}

func conditionStatusForMulticloudResource(condition shared.OSOKConditionType) corev1.ConditionStatus {
	if condition == shared.Failed {
		return corev1.ConditionFalse
	}
	return corev1.ConditionTrue
}

func annotationValue(annotations map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(annotations[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
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
	return value.UTC().Format(time.RFC3339Nano)
}

func copyStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	copied := make(map[string]string, len(source))
	for key, value := range source {
		copied[key] = value
	}
	return copied
}

func copyMapValue(source map[string]map[string]interface{}) map[string]shared.MapValue {
	if len(source) == 0 {
		return nil
	}
	copied := make(map[string]shared.MapValue, len(source))
	for namespace, values := range source {
		if len(values) == 0 {
			continue
		}
		namespaceValues := make(shared.MapValue, len(values))
		for key, value := range values {
			if stringValue, ok := value.(string); ok {
				namespaceValues[key] = stringValue
			}
		}
		if len(namespaceValues) > 0 {
			copied[namespace] = namespaceValues
		}
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}
