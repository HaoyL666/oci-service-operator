/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package resourceanchor

import (
	"context"
	"encoding/json"
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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	resourceAnchorIDAnnotation                      = "multicloud.oracle.com/resource-anchor-id"
	resourceAnchorSubscriptionIDAnnotation          = "multicloud.oracle.com/subscription-id"
	resourceAnchorSubscriptionServiceNameAnnotation = "multicloud.oracle.com/subscription-service-name"
	resourceAnchorCompartmentIDAnnotation           = "multicloud.oracle.com/compartment-id"
	resourceAnchorLinkedCompartmentIDAnnotation     = "multicloud.oracle.com/linked-compartment-id"
	resourceAnchorDisplayNameAnnotation             = "multicloud.oracle.com/display-name"
	resourceAnchorCSPResourceAnchorIDAnnotation     = "multicloud.oracle.com/csp-resource-anchor-id"
	resourceAnchorCSPResourceAnchorNameAnnotation   = "multicloud.oracle.com/csp-resource-anchor-name"

	resourceAnchorLegacyIDAnnotation                      = "multicloud.oracle.com/resourceAnchorId"
	resourceAnchorLegacySubscriptionIDAnnotation          = "multicloud.oracle.com/subscriptionId"
	resourceAnchorLegacySubscriptionServiceNameAnnotation = "multicloud.oracle.com/subscriptionServiceName"
)

type resourceAnchorGetCall func(context.Context, multicloudsdk.GetResourceAnchorRequest) (multicloudsdk.GetResourceAnchorResponse, error)
type resourceAnchorListCall func(context.Context, multicloudsdk.ListResourceAnchorsRequest) (multicloudsdk.ListResourceAnchorsResponse, error)

type resourceAnchorReadOnlyClient struct {
	delegate ResourceAnchorServiceClient
	get      resourceAnchorGetCall
	list     resourceAnchorListCall
	log      loggerutil.OSOKLogger
}

type resourceAnchorIdentity struct {
	resourceAnchorID        string
	subscriptionID          string
	subscriptionServiceName string
	compartmentID           string
	linkedCompartmentID     string
	displayName             string
	cspResourceAnchorID     string
	cspResourceAnchorName   string
}

func init() {
	registerResourceAnchorRuntimeHooksMutator(func(manager *ResourceAnchorServiceManager, hooks *ResourceAnchorRuntimeHooks) {
		get := hooks.Get.Call
		list := hooks.List.Call
		hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(delegate ResourceAnchorServiceClient) ResourceAnchorServiceClient {
			return resourceAnchorReadOnlyClient{
				delegate: delegate,
				get:      get,
				list:     list,
				log:      manager.Log,
			}
		})
	})
}

func (c resourceAnchorReadOnlyClient) CreateOrUpdate(ctx context.Context, resource *multicloudv1beta1.ResourceAnchor, _ ctrl.Request) (servicemanager.OSOKResponse, error) {
	identity, err := resolveResourceAnchorIdentity(resource)
	if err != nil {
		return failResourceAnchor(resource, err, c.log)
	}

	if identity.canGet() {
		response, err := c.getResourceAnchor(ctx, identity)
		if err != nil {
			return failResourceAnchor(resource, err, c.log)
		}
		projectResourceAnchor(resource, response.ResourceAnchor)
		servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)
		return applyResourceAnchorStatus(resource, c.log), nil
	}

	summary, response, err := c.findResourceAnchor(ctx, identity)
	if err != nil {
		return failResourceAnchor(resource, err, c.log)
	}
	projectResourceAnchorSummary(resource, summary)
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)
	return applyResourceAnchorStatus(resource, c.log), nil
}

func (c resourceAnchorReadOnlyClient) Delete(ctx context.Context, resource *multicloudv1beta1.ResourceAnchor) (bool, error) {
	return c.delegate.Delete(ctx, resource)
}

func (c resourceAnchorReadOnlyClient) getResourceAnchor(ctx context.Context, identity resourceAnchorIdentity) (multicloudsdk.GetResourceAnchorResponse, error) {
	if c.get == nil {
		return multicloudsdk.GetResourceAnchorResponse{}, fmt.Errorf("ResourceAnchor read requires GetResourceAnchor, but the OCI client is not configured")
	}

	serviceName, err := getResourceAnchorSubscriptionServiceName(identity.subscriptionServiceName)
	if err != nil {
		return multicloudsdk.GetResourceAnchorResponse{}, err
	}
	return c.get(ctx, multicloudsdk.GetResourceAnchorRequest{
		ResourceAnchorId:           common.String(identity.resourceAnchorID),
		SubscriptionId:             common.String(identity.subscriptionID),
		SubscriptionServiceName:    serviceName,
		ShouldFetchCompartmentName: common.Bool(true),
	})
}

func (c resourceAnchorReadOnlyClient) findResourceAnchor(ctx context.Context, identity resourceAnchorIdentity) (multicloudsdk.ResourceAnchorSummary, multicloudsdk.ListResourceAnchorsResponse, error) {
	if c.list == nil {
		return multicloudsdk.ResourceAnchorSummary{}, multicloudsdk.ListResourceAnchorsResponse{}, fmt.Errorf("ResourceAnchor bind requires ListResourceAnchors, but the OCI client is not configured")
	}
	if !identity.hasListCriteria() {
		return multicloudsdk.ResourceAnchorSummary{}, multicloudsdk.ListResourceAnchorsResponse{}, fmt.Errorf("ResourceAnchor is read-only in the multicloud SDK; provide %q or list filter annotations to bind an existing OCI resource", resourceAnchorIDAnnotation)
	}

	request, err := listResourceAnchorsRequest(identity)
	if err != nil {
		return multicloudsdk.ResourceAnchorSummary{}, multicloudsdk.ListResourceAnchorsResponse{}, err
	}
	return c.findResourceAnchorWithRequest(ctx, identity, request)
}

func (c resourceAnchorReadOnlyClient) findResourceAnchorWithRequest(ctx context.Context, identity resourceAnchorIdentity, request multicloudsdk.ListResourceAnchorsRequest) (multicloudsdk.ResourceAnchorSummary, multicloudsdk.ListResourceAnchorsResponse, error) {
	var match resourceAnchorListMatch
	for {
		response, err := c.list(ctx, request)
		if err != nil {
			return multicloudsdk.ResourceAnchorSummary{}, multicloudsdk.ListResourceAnchorsResponse{}, err
		}
		match, err = match.add(response, identity)
		if err != nil {
			return multicloudsdk.ResourceAnchorSummary{}, multicloudsdk.ListResourceAnchorsResponse{}, err
		}
		if !hasNextResourceAnchorListPage(response) {
			break
		}
		request.Page = response.OpcNextPage
	}

	if !match.found {
		return multicloudsdk.ResourceAnchorSummary{}, multicloudsdk.ListResourceAnchorsResponse{}, fmt.Errorf("ResourceAnchor list lookup did not find a matching OCI resource")
	}
	return match.summary, match.response, nil
}

type resourceAnchorListMatch struct {
	summary  multicloudsdk.ResourceAnchorSummary
	response multicloudsdk.ListResourceAnchorsResponse
	found    bool
}

func (m resourceAnchorListMatch) add(response multicloudsdk.ListResourceAnchorsResponse, identity resourceAnchorIdentity) (resourceAnchorListMatch, error) {
	for i := range response.Items {
		item := response.Items[i]
		if !resourceAnchorSummaryMatches(item, identity) {
			continue
		}
		if m.found {
			return m, fmt.Errorf("ResourceAnchor list lookup returned multiple matching resources; add %q to disambiguate", resourceAnchorIDAnnotation)
		}
		m.summary = item
		m.response = response
		m.found = true
	}
	return m, nil
}

func hasNextResourceAnchorListPage(response multicloudsdk.ListResourceAnchorsResponse) bool {
	return stringValue(response.OpcNextPage) != ""
}

func resolveResourceAnchorIdentity(resource *multicloudv1beta1.ResourceAnchor) (resourceAnchorIdentity, error) {
	if resource == nil {
		return resourceAnchorIdentity{}, fmt.Errorf("ResourceAnchor resource is nil")
	}

	annotations := resource.GetAnnotations()
	status := resource.Status

	recordedID := firstNonEmptyString(string(status.OsokStatus.Ocid), status.Id)
	annotatedID := annotationValue(annotations, resourceAnchorIDAnnotation, resourceAnchorLegacyIDAnnotation)
	resourceAnchorID, err := resolveCreateOnlyAnnotation(resourceAnchorIDAnnotation, "identity", "recorded OCI ID", recordedID, annotatedID)
	if err != nil {
		return resourceAnchorIdentity{}, err
	}

	recordedSubscriptionID := status.SubscriptionId
	annotatedSubscriptionID := annotationValue(annotations, resourceAnchorSubscriptionIDAnnotation, resourceAnchorLegacySubscriptionIDAnnotation)
	subscriptionID, err := resolveCreateOnlyAnnotation(resourceAnchorSubscriptionIDAnnotation, "subscription", "recorded subscription", recordedSubscriptionID, annotatedSubscriptionID)
	if err != nil {
		return resourceAnchorIdentity{}, err
	}

	recordedServiceName := strings.ToUpper(strings.TrimSpace(status.SubscriptionType))
	annotatedServiceName := strings.ToUpper(annotationValue(annotations, resourceAnchorSubscriptionServiceNameAnnotation, resourceAnchorLegacySubscriptionServiceNameAnnotation))
	serviceName, err := resolveCreateOnlyAnnotation(resourceAnchorSubscriptionServiceNameAnnotation, "subscription service", "recorded service", recordedServiceName, annotatedServiceName)
	if err != nil {
		return resourceAnchorIdentity{}, err
	}

	return resourceAnchorIdentity{
		resourceAnchorID:        resourceAnchorID,
		subscriptionID:          subscriptionID,
		subscriptionServiceName: serviceName,
		compartmentID:           firstNonEmptyString(status.CompartmentId, annotationValue(annotations, resourceAnchorCompartmentIDAnnotation)),
		linkedCompartmentID:     firstNonEmptyString(status.LinkedCompartmentId, annotationValue(annotations, resourceAnchorLinkedCompartmentIDAnnotation)),
		displayName:             firstNonEmptyString(annotationValue(annotations, resourceAnchorDisplayNameAnnotation), status.DisplayName),
		cspResourceAnchorID:     firstNonEmptyString(annotationValue(annotations, resourceAnchorCSPResourceAnchorIDAnnotation), status.CspResourceAnchorId),
		cspResourceAnchorName:   firstNonEmptyString(annotationValue(annotations, resourceAnchorCSPResourceAnchorNameAnnotation), status.CspResourceAnchorName),
	}, nil
}

func resolveCreateOnlyAnnotation(annotation string, description string, recordedLabel string, recordedValue string, annotatedValue string) (string, error) {
	if recordedValue != "" && annotatedValue != "" && recordedValue != annotatedValue {
		return "", fmt.Errorf("ResourceAnchor create-only %s annotation %q changed from %s %q to %q; create a replacement resource instead", description, annotation, recordedLabel, recordedValue, annotatedValue)
	}
	return firstNonEmptyString(recordedValue, annotatedValue), nil
}

func (i resourceAnchorIdentity) canGet() bool {
	return i.resourceAnchorID != "" && i.subscriptionID != "" && i.subscriptionServiceName != ""
}

func (i resourceAnchorIdentity) hasListCriteria() bool {
	return i.resourceAnchorID != "" ||
		i.subscriptionID != "" ||
		i.subscriptionServiceName != "" ||
		i.compartmentID != "" ||
		i.linkedCompartmentID != "" ||
		i.displayName != "" ||
		i.cspResourceAnchorID != "" ||
		i.cspResourceAnchorName != ""
}

func listResourceAnchorsRequest(identity resourceAnchorIdentity) (multicloudsdk.ListResourceAnchorsRequest, error) {
	request := multicloudsdk.ListResourceAnchorsRequest{
		CompartmentId:              optionalString(identity.compartmentID),
		LinkedCompartmentId:        optionalString(identity.linkedCompartmentID),
		DisplayName:                optionalString(identity.displayName),
		Id:                         optionalString(identity.resourceAnchorID),
		SubscriptionId:             optionalString(identity.subscriptionID),
		ShouldFetchCompartmentName: common.Bool(true),
	}
	if identity.subscriptionServiceName != "" {
		serviceName, err := listResourceAnchorSubscriptionServiceName(identity.subscriptionServiceName)
		if err != nil {
			return multicloudsdk.ListResourceAnchorsRequest{}, err
		}
		request.SubscriptionServiceName = serviceName
	}
	return request, nil
}

func resourceAnchorSummaryMatches(item multicloudsdk.ResourceAnchorSummary, identity resourceAnchorIdentity) bool {
	compared := false
	matches := func(expected string, actual *string) bool {
		if expected == "" {
			return true
		}
		compared = true
		return strings.TrimSpace(expected) == strings.TrimSpace(stringValue(actual))
	}
	if !matches(identity.resourceAnchorID, item.Id) ||
		!matches(identity.subscriptionID, item.SubscriptionId) ||
		!matches(identity.compartmentID, item.CompartmentId) ||
		!matches(identity.linkedCompartmentID, item.LinkedCompartmentId) ||
		!matches(identity.displayName, item.DisplayName) ||
		!matches(identity.cspResourceAnchorID, item.CspResourceAnchorId) ||
		!matches(identity.cspResourceAnchorName, item.CspResourceAnchorName) {
		return false
	}
	if identity.subscriptionServiceName != "" {
		compared = true
	}
	return compared
}

func projectResourceAnchor(resource *multicloudv1beta1.ResourceAnchor, current multicloudsdk.ResourceAnchor) {
	status := &resource.Status
	status.Id = stringValue(current.Id)
	status.DisplayName = stringValue(current.DisplayName)
	status.CompartmentId = stringValue(current.CompartmentId)
	status.TimeCreated = sdkTimeString(current.TimeCreated)
	status.LifecycleState = string(current.LifecycleState)
	status.FreeformTags = cloneStringMap(current.FreeformTags)
	status.DefinedTags = mapInterfaceTags(current.DefinedTags)
	status.SystemTags = mapInterfaceTags(current.SystemTags)
	status.SubscriptionId = stringValue(current.SubscriptionId)
	status.Region = stringValue(current.Region)
	status.CompartmentName = stringValue(current.CompartmentName)
	status.TimeUpdated = sdkTimeString(current.TimeUpdated)
	status.LifecycleDetails = stringValue(current.LifecycleDetails)
	status.SetupMode = string(current.SetupMode)
	status.LinkedCompartmentId = stringValue(current.LinkedCompartmentId)
	status.LinkedCompartmentName = stringValue(current.LinkedCompartmentName)
	status.SubscriptionType = string(current.SubscriptionType)
	projectResourceAnchorMetadata(status, current.CloudServiceProviderMetadataItem)
}

func projectResourceAnchorSummary(resource *multicloudv1beta1.ResourceAnchor, summary multicloudsdk.ResourceAnchorSummary) {
	status := &resource.Status
	status.Id = stringValue(summary.Id)
	status.DisplayName = stringValue(summary.DisplayName)
	status.CompartmentId = stringValue(summary.CompartmentId)
	status.TimeCreated = sdkTimeString(summary.TimeCreated)
	status.LifecycleState = string(summary.LifecycleState)
	status.FreeformTags = cloneStringMap(summary.FreeformTags)
	status.DefinedTags = mapInterfaceTags(summary.DefinedTags)
	status.SystemTags = mapInterfaceTags(summary.SystemTags)
	status.SubscriptionId = stringValue(summary.SubscriptionId)
	status.CompartmentName = stringValue(summary.CompartmentName)
	status.PartnerCloudAccountIdentifier = stringValue(summary.PartnerCloudAccountIdentifier)
	status.CspResourceAnchorId = stringValue(summary.CspResourceAnchorId)
	status.CspResourceAnchorName = stringValue(summary.CspResourceAnchorName)
	status.CspAdditionalProperties = cloneStringMap(summary.CspAdditionalProperties)
	status.TimeUpdated = sdkTimeString(summary.TimeUpdated)
	status.LifecycleDetails = stringValue(summary.LifecycleDetails)
	status.LinkedCompartmentId = stringValue(summary.LinkedCompartmentId)
	status.LinkedCompartmentName = stringValue(summary.LinkedCompartmentName)
}

func projectResourceAnchorMetadata(status *multicloudv1beta1.ResourceAnchorStatus, item multicloudsdk.CloudServiceProviderMetadataItem) {
	if item == nil {
		return
	}

	status.CloudServiceProviderMetadataItem.Region = stringValue(item.GetRegion())
	status.CloudServiceProviderMetadataItem.CspResourceAnchorId = stringValue(item.GetCspResourceAnchorId())
	status.CloudServiceProviderMetadataItem.CspResourceAnchorName = stringValue(item.GetCspResourceAnchorName())
	status.CloudServiceProviderMetadataItem.ResourceAnchorUri = stringValue(item.GetResourceAnchorUri())
	status.CloudServiceProviderMetadataItem.CspAdditionalProperties = cloneStringMap(item.GetCspAdditionalProperties())
	status.CloudServiceProviderMetadataItem.ResourceAnchorName = stringValue(item.GetResourceAnchorName())
	status.CloudServiceProviderMetadataItem.SubscriptionType = status.SubscriptionType
	status.CspResourceAnchorId = status.CloudServiceProviderMetadataItem.CspResourceAnchorId
	status.CspResourceAnchorName = status.CloudServiceProviderMetadataItem.CspResourceAnchorName
	status.CspAdditionalProperties = cloneStringMap(status.CloudServiceProviderMetadataItem.CspAdditionalProperties)

	if payload, err := json.Marshal(item); err == nil {
		status.CloudServiceProviderMetadataItem.JsonData = string(payload)
	}
}

func applyResourceAnchorStatus(resource *multicloudv1beta1.ResourceAnchor, log loggerutil.OSOKLogger) servicemanager.OSOKResponse {
	status := &resource.Status.OsokStatus
	now := metav1.Now()
	status.UpdatedAt = &now
	if resource.Status.Id != "" {
		status.Ocid = shared.OCID(resource.Status.Id)
		if status.CreatedAt == nil {
			status.CreatedAt = &now
		}
	}

	message := firstNonEmptyString(resource.Status.LifecycleDetails, resource.Status.DisplayName, "OCI ResourceAnchor is active")
	if current := servicemanager.NewLifecycleAsyncOperation(status, resource.Status.LifecycleState, message, shared.OSOKAsyncPhaseCreate); current != nil {
		projection := servicemanager.ApplyAsyncOperation(status, current, log)
		return servicemanager.OSOKResponse{
			IsSuccessful:  projection.Condition != shared.Failed,
			ShouldRequeue: projection.ShouldRequeue,
		}
	}

	servicemanager.ClearAsyncOperation(status)
	status.Message = message
	status.Reason = string(shared.Active)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Active, v1.ConditionTrue, "", message, log)
	return servicemanager.OSOKResponse{IsSuccessful: true}
}

func failResourceAnchor(resource *multicloudv1beta1.ResourceAnchor, err error, log loggerutil.OSOKLogger) (servicemanager.OSOKResponse, error) {
	if resource == nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	status := &resource.Status.OsokStatus
	servicemanager.RecordErrorOpcRequestID(status, err)
	now := metav1.Now()
	status.UpdatedAt = &now
	status.Message = err.Error()
	status.Reason = string(shared.Failed)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Failed, v1.ConditionFalse, "", err.Error(), log)
	return servicemanager.OSOKResponse{IsSuccessful: false}, err
}

func getResourceAnchorSubscriptionServiceName(value string) (multicloudsdk.GetResourceAnchorSubscriptionServiceNameEnum, error) {
	if serviceName, ok := multicloudsdk.GetMappingGetResourceAnchorSubscriptionServiceNameEnum(value); ok {
		return serviceName, nil
	}
	return "", fmt.Errorf("ResourceAnchor subscription service name %q is not supported; supported values are %s", value, strings.Join(multicloudsdk.GetGetResourceAnchorSubscriptionServiceNameEnumStringValues(), ","))
}

func listResourceAnchorSubscriptionServiceName(value string) (multicloudsdk.ListResourceAnchorsSubscriptionServiceNameEnum, error) {
	if serviceName, ok := multicloudsdk.GetMappingListResourceAnchorsSubscriptionServiceNameEnum(value); ok {
		return serviceName, nil
	}
	return "", fmt.Errorf("ResourceAnchor subscription service name %q is not supported; supported values are %s", value, strings.Join(multicloudsdk.GetListResourceAnchorsSubscriptionServiceNameEnumStringValues(), ","))
}

func annotationValue(annotations map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(annotations[key]); value != "" {
			return value
		}
	}
	return ""
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return common.String(value)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func mapInterfaceTags(in map[string]map[string]interface{}) map[string]shared.MapValue {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]shared.MapValue, len(in))
	for namespace, values := range in {
		if len(values) == 0 {
			out[namespace] = shared.MapValue{}
			continue
		}
		out[namespace] = make(shared.MapValue, len(values))
		for key, value := range values {
			out[namespace][key] = fmt.Sprint(value)
		}
	}
	return out
}

func sdkTimeString(value *common.SDKTime) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
