/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package networkanchor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	multicloudsdk "github.com/oracle/oci-go-sdk/v65/multicloud"
	multicloudv1beta1 "github.com/oracle/oci-service-operator/api/multicloud/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	networkAnchorRequeueDuration = time.Minute

	networkAnchorIDAnnotation                     = "multicloud.oracle.com/network-anchor-id"
	networkAnchorSubscriptionIDAnnotation         = "multicloud.oracle.com/network-anchor-subscription-id"
	networkAnchorSubscriptionServiceAnnotation    = "multicloud.oracle.com/network-anchor-subscription-service-name"
	networkAnchorExternalLocationAnnotation       = "multicloud.oracle.com/network-anchor-external-location"
	networkAnchorCompartmentIDAnnotation          = "multicloud.oracle.com/network-anchor-compartment-id"
	networkAnchorDisplayNameAnnotation            = "multicloud.oracle.com/network-anchor-display-name"
	networkAnchorOciVCNIDAnnotation               = "multicloud.oracle.com/network-anchor-oci-vcn-id"
	networkAnchorOciSubnetIDAnnotation            = "multicloud.oracle.com/network-anchor-oci-subnet-id"
	networkAnchorCompartmentIDInSubtreeAnnotation = "multicloud.oracle.com/network-anchor-compartment-id-in-subtree"
	networkAnchorShouldFetchVCNNameAnnotation     = "multicloud.oracle.com/network-anchor-should-fetch-vcn-name"
)

type networkAnchorOCIClient interface {
	GetNetworkAnchor(context.Context, multicloudsdk.GetNetworkAnchorRequest) (multicloudsdk.GetNetworkAnchorResponse, error)
	ListNetworkAnchors(context.Context, multicloudsdk.ListNetworkAnchorsRequest) (multicloudsdk.ListNetworkAnchorsResponse, error)
}

type networkAnchorRuntimeClient struct {
	log     loggerutil.OSOKLogger
	client  networkAnchorOCIClient
	initErr error
}

type networkAnchorBindTarget struct {
	id                     string
	subscriptionID         string
	subscriptionService    string
	externalLocation       string
	compartmentID          string
	displayName            string
	ociVCNID               string
	ociSubnetID            string
	compartmentIDInSubtree *bool
	shouldFetchVCNName     *bool
}

type networkAnchorObservation struct {
	response any
	get      *multicloudsdk.GetNetworkAnchorResponse
	summary  *multicloudsdk.NetworkAnchorSummary
}

var _ NetworkAnchorServiceClient = (*networkAnchorRuntimeClient)(nil)

func init() {
	registerNetworkAnchorRuntimeHooksMutator(func(manager *NetworkAnchorServiceManager, hooks *NetworkAnchorRuntimeHooks) {
		applyNetworkAnchorRuntimeHooks(manager, hooks)
	})
}

func applyNetworkAnchorRuntimeHooks(manager *NetworkAnchorServiceManager, hooks *NetworkAnchorRuntimeHooks) {
	if hooks == nil {
		return
	}

	hooks.Semantics = newNetworkAnchorRuntimeSemantics()
	if listCall := hooks.List.Call; listCall != nil {
		hooks.List.Call = func(ctx context.Context, request multicloudsdk.ListNetworkAnchorsRequest) (multicloudsdk.ListNetworkAnchorsResponse, error) {
			return listNetworkAnchorsAllPages(ctx, listCall, request)
		}
	}

	var (
		log      loggerutil.OSOKLogger
		provider common.ConfigurationProvider
	)
	if manager != nil {
		log = manager.Log
		provider = manager.Provider
	}
	sdkClient, initErr := multicloudsdk.NewOmhubNetworkAnchorClientWithConfigurationProvider(provider)
	hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(_ NetworkAnchorServiceClient) NetworkAnchorServiceClient {
		return newNetworkAnchorRuntimeClient(log, sdkClient, initErr)
	})
}

func newNetworkAnchorRuntimeClient(log loggerutil.OSOKLogger, client networkAnchorOCIClient, initErr error) NetworkAnchorServiceClient {
	if initErr != nil {
		initErr = fmt.Errorf("initialize NetworkAnchor OCI client: %w", initErr)
	}
	return &networkAnchorRuntimeClient{
		log:     log,
		client:  client,
		initErr: initErr,
	}
}

func newNetworkAnchorRuntimeSemantics() *generatedruntime.Semantics {
	return &generatedruntime.Semantics{
		FormalService:       "multicloud",
		FormalSlug:          "networkanchor",
		StatusProjection:    "required",
		SecretSideEffects:   "none",
		FinalizerPolicy:     "unbind-only",
		AuxiliaryOperations: []generatedruntime.AuxiliaryOperation{},
		Unsupported:         []generatedruntime.UnsupportedSemantic{},
		Async: &generatedruntime.AsyncSemantics{
			Strategy:             "lifecycle",
			Runtime:              "generatedruntime",
			FormalClassification: "lifecycle",
		},
		Lifecycle: generatedruntime.LifecycleSemantics{
			ProvisioningStates: []string{string(multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateCreating)},
			UpdatingStates:     []string{string(multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateUpdating)},
			ActiveStates:       []string{string(multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateActive)},
		},
		Delete: generatedruntime.DeleteSemantics{
			Policy:         "best-effort",
			PendingStates:  []string{string(multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateDeleting)},
			TerminalStates: []string{string(multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateDeleted)},
		},
		List: &generatedruntime.ListSemantics{
			ResponseItemsField: "Items",
			MatchFields:        []string{"id", "displayName", "compartmentId", "networkAnchorOciVcnId", "networkAnchorOciSubnetId"},
		},
		Mutation: generatedruntime.MutationSemantics{
			Mutable:       []string{},
			ForceNew:      []string{},
			ConflictsWith: map[string][]string{},
		},
	}
}

func (c *networkAnchorRuntimeClient) CreateOrUpdate(ctx context.Context, resource *multicloudv1beta1.NetworkAnchor, _ ctrl.Request) (servicemanager.OSOKResponse, error) {
	if resource == nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, fmt.Errorf("NetworkAnchor resource is nil")
	}
	if c.initErr != nil {
		return c.fail(resource, c.initErr)
	}
	if c.client == nil {
		return c.fail(resource, fmt.Errorf("NetworkAnchor OCI client is nil"))
	}

	target, err := networkAnchorBindTargetFromResource(resource)
	if err != nil {
		return c.fail(resource, err)
	}
	if err := target.validate(); err != nil {
		return c.fail(resource, err)
	}

	observation, err := c.observeNetworkAnchor(ctx, target)
	if observation.response != nil {
		servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, observation.response)
	}
	if err != nil {
		return c.fail(resource, err)
	}
	lifecycle := observation.project(resource)
	return c.applyLifecycle(resource, lifecycle)
}

func (c *networkAnchorRuntimeClient) observeNetworkAnchor(ctx context.Context, target networkAnchorBindTarget) (networkAnchorObservation, error) {
	if target.canGet() {
		observation, found, err := c.getNetworkAnchor(ctx, target)
		if err != nil || found {
			return observation, err
		}
	}
	return c.listNetworkAnchor(ctx, target)
}

func (c *networkAnchorRuntimeClient) getNetworkAnchor(ctx context.Context, target networkAnchorBindTarget) (networkAnchorObservation, bool, error) {
	response, err := c.client.GetNetworkAnchor(ctx, target.getRequest())
	if err == nil {
		return networkAnchorObservation{response: response, get: &response}, true, nil
	}
	if networkAnchorAuthShapedNotFound(err) {
		return networkAnchorObservation{}, false, err
	}
	if networkAnchorOrdinaryNotFound(err) {
		return networkAnchorObservation{}, false, nil
	}
	return networkAnchorObservation{}, false, err
}

func (c *networkAnchorRuntimeClient) listNetworkAnchor(ctx context.Context, target networkAnchorBindTarget) (networkAnchorObservation, error) {
	response, err := listNetworkAnchorsAllPages(ctx, c.client.ListNetworkAnchors, target.listRequest())
	observation := networkAnchorObservation{response: response}
	if err != nil {
		return observation, err
	}
	matches := matchingNetworkAnchorSummaries(target, response.Items)
	switch len(matches) {
	case 0:
		return observation, fmt.Errorf("no NetworkAnchor matched the configured bind criteria")
	case 1:
		observation.summary = &matches[0]
		return observation, nil
	default:
		return observation, fmt.Errorf("multiple NetworkAnchors matched the configured bind criteria; add %q or narrow the filters", networkAnchorIDAnnotation)
	}
}

func (observation networkAnchorObservation) project(resource *multicloudv1beta1.NetworkAnchor) string {
	if observation.get != nil {
		projectNetworkAnchorFromGet(resource, *observation.get)
		return string(observation.get.NetworkAnchorLifecycleState)
	}
	if observation.summary != nil {
		projectNetworkAnchorFromSummary(resource, *observation.summary)
		return string(observation.summary.NetworkAnchorLifecycleState)
	}
	return ""
}

func (c *networkAnchorRuntimeClient) Delete(_ context.Context, resource *multicloudv1beta1.NetworkAnchor) (bool, error) {
	if resource == nil {
		return false, fmt.Errorf("NetworkAnchor resource is nil")
	}

	message := "NetworkAnchor delete is not supported by the current multicloud SDK; releasing the Kubernetes finalizer without deleting the OCI resource"
	now := metav1.Now()
	status := &resource.Status.OsokStatus
	status.DeletedAt = &now
	status.UpdatedAt = &now
	status.Message = message
	status.Reason = string(shared.Terminating)
	servicemanager.ClearAsyncOperation(status)
	resource.Status.OsokStatus = util.UpdateOSOKStatusCondition(resource.Status.OsokStatus, shared.Terminating, corev1.ConditionTrue, "", message, c.log)
	return true, nil
}

func (c *networkAnchorRuntimeClient) applyLifecycle(resource *multicloudv1beta1.NetworkAnchor, lifecycle string) (servicemanager.OSOKResponse, error) {
	status := &resource.Status.OsokStatus
	now := metav1.Now()
	if status.Ocid != "" && status.CreatedAt == nil {
		status.CreatedAt = &now
	}
	status.UpdatedAt = &now

	condition, phase, class, shouldRequeue := networkAnchorLifecycleProjection(lifecycle)
	message := networkAnchorLifecycleMessage(condition, lifecycle, resource.Status.LifecycleDetails)
	status.Message = message
	status.Reason = string(condition)

	if shouldRequeue {
		projection := servicemanager.ApplyAsyncOperation(status, &shared.OSOKAsyncOperation{
			Source:          shared.OSOKAsyncSourceLifecycle,
			Phase:           phase,
			RawStatus:       lifecycle,
			NormalizedClass: class,
			Message:         message,
			UpdatedAt:       &now,
		}, c.log)
		return servicemanager.OSOKResponse{
			IsSuccessful:    projection.Condition != shared.Failed,
			ShouldRequeue:   projection.ShouldRequeue,
			RequeueDuration: networkAnchorRequeueDuration,
		}, nil
	}

	servicemanager.ClearAsyncOperation(status)
	conditionStatus := corev1.ConditionTrue
	if condition == shared.Failed {
		conditionStatus = corev1.ConditionFalse
	}
	resource.Status.OsokStatus = util.UpdateOSOKStatusCondition(resource.Status.OsokStatus, condition, conditionStatus, "", message, c.log)
	return servicemanager.OSOKResponse{
		IsSuccessful:  condition != shared.Failed,
		ShouldRequeue: false,
	}, nil
}

func (c *networkAnchorRuntimeClient) fail(resource *multicloudv1beta1.NetworkAnchor, err error) (servicemanager.OSOKResponse, error) {
	if resource == nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	status := &resource.Status.OsokStatus
	servicemanager.RecordErrorOpcRequestID(status, err)
	status.Message = err.Error()
	status.Reason = string(shared.Failed)
	now := metav1.Now()
	status.UpdatedAt = &now
	if status.Async.Current != nil {
		current := *status.Async.Current
		current.NormalizedClass = shared.OSOKAsyncClassFailed
		current.Message = err.Error()
		current.UpdatedAt = &now
		servicemanager.ApplyAsyncOperation(status, &current, c.log)
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	resource.Status.OsokStatus = util.UpdateOSOKStatusCondition(resource.Status.OsokStatus, shared.Failed, corev1.ConditionFalse, "", err.Error(), c.log)
	return servicemanager.OSOKResponse{IsSuccessful: false}, err
}

func networkAnchorBindTargetFromResource(resource *multicloudv1beta1.NetworkAnchor) (networkAnchorBindTarget, error) {
	target := networkAnchorBindTarget{
		id:                  firstNonEmptyNetworkAnchorString(networkAnchorAnnotation(resource, networkAnchorIDAnnotation), string(resource.Status.OsokStatus.Ocid), resource.Status.Id),
		subscriptionID:      networkAnchorAnnotation(resource, networkAnchorSubscriptionIDAnnotation),
		subscriptionService: firstNonEmptyNetworkAnchorString(networkAnchorAnnotation(resource, networkAnchorSubscriptionServiceAnnotation), resource.Status.SubscriptionType),
		externalLocation:    firstNonEmptyNetworkAnchorString(networkAnchorAnnotation(resource, networkAnchorExternalLocationAnnotation), resource.Status.CloudServiceProviderMetadataItem.Region),
		compartmentID:       firstNonEmptyNetworkAnchorString(networkAnchorAnnotation(resource, networkAnchorCompartmentIDAnnotation), resource.Status.CompartmentId),
		displayName:         firstNonEmptyNetworkAnchorString(networkAnchorAnnotation(resource, networkAnchorDisplayNameAnnotation), resource.Status.DisplayName, resource.Name),
		ociVCNID:            firstNonEmptyNetworkAnchorString(networkAnchorAnnotation(resource, networkAnchorOciVCNIDAnnotation), resource.Status.VcnId, resource.Status.OciMetadataItem.Vcn.VcnId),
		ociSubnetID:         firstNonEmptyNetworkAnchorString(networkAnchorAnnotation(resource, networkAnchorOciSubnetIDAnnotation), firstNetworkAnchorSubnetID(resource)),
	}

	compartmentIDInSubtree, err := networkAnchorBoolAnnotation(resource, networkAnchorCompartmentIDInSubtreeAnnotation)
	if err != nil {
		return target, err
	}
	target.compartmentIDInSubtree = compartmentIDInSubtree

	shouldFetchVCNName, err := networkAnchorBoolAnnotation(resource, networkAnchorShouldFetchVCNNameAnnotation)
	if err != nil {
		return target, err
	}
	if shouldFetchVCNName == nil {
		target.shouldFetchVCNName = common.Bool(true)
	} else {
		target.shouldFetchVCNName = shouldFetchVCNName
	}

	return target, nil
}

func (target networkAnchorBindTarget) validate() error {
	if err := target.validateSubscriptionService(); err != nil {
		return err
	}
	if target.id != "" {
		return nil
	}
	if target.displayName != "" && (target.compartmentID != "" || target.subscriptionService != "") {
		return nil
	}
	if (target.ociVCNID != "" || target.ociSubnetID != "") && (target.compartmentID != "" || target.subscriptionService != "") {
		return nil
	}
	return fmt.Errorf("NetworkAnchor uses a read-only multicloud SDK surface and spec has no create fields; provide an existing OCI identity or safe bind filters using annotations such as %q with %q/%q", networkAnchorIDAnnotation, networkAnchorSubscriptionIDAnnotation, networkAnchorSubscriptionServiceAnnotation)
}

func (target networkAnchorBindTarget) validateSubscriptionService() error {
	if target.subscriptionService == "" {
		return nil
	}
	if _, ok := multicloudsdk.GetMappingListNetworkAnchorsSubscriptionServiceNameEnum(target.subscriptionService); ok {
		return nil
	}
	return fmt.Errorf("NetworkAnchor annotation %q has unsupported subscription service %q; supported values are %s", networkAnchorSubscriptionServiceAnnotation, target.subscriptionService, strings.Join(multicloudsdk.GetListNetworkAnchorsSubscriptionServiceNameEnumStringValues(), ","))
}

func (target networkAnchorBindTarget) canGet() bool {
	return target.id != "" && target.subscriptionID != "" && target.subscriptionService != ""
}

func (target networkAnchorBindTarget) getRequest() multicloudsdk.GetNetworkAnchorRequest {
	request := multicloudsdk.GetNetworkAnchorRequest{
		NetworkAnchorId:    common.String(target.id),
		SubscriptionId:     common.String(target.subscriptionID),
		ExternalLocation:   stringPtrOrNil(target.externalLocation),
		ShouldFetchVcnName: target.shouldFetchVCNName,
	}
	if value, ok := multicloudsdk.GetMappingGetNetworkAnchorSubscriptionServiceNameEnum(target.subscriptionService); ok {
		request.SubscriptionServiceName = value
	}
	return request
}

func (target networkAnchorBindTarget) listRequest() multicloudsdk.ListNetworkAnchorsRequest {
	request := multicloudsdk.ListNetworkAnchorsRequest{
		CompartmentId:            stringPtrOrNil(target.compartmentID),
		SubscriptionId:           stringPtrOrNil(target.subscriptionID),
		DisplayName:              stringPtrOrNil(target.displayName),
		ExternalLocation:         stringPtrOrNil(target.externalLocation),
		NetworkAnchorOciVcnId:    stringPtrOrNil(target.ociVCNID),
		NetworkAnchorOciSubnetId: stringPtrOrNil(target.ociSubnetID),
		Id:                       stringPtrOrNil(target.id),
		CompartmentIdInSubtree:   target.compartmentIDInSubtree,
		ShouldFetchVcnName:       target.shouldFetchVCNName,
		Limit:                    common.Int(100),
	}
	if value, ok := multicloudsdk.GetMappingListNetworkAnchorsSubscriptionServiceNameEnum(target.subscriptionService); ok {
		request.SubscriptionServiceName = value
	}
	return request
}

func listNetworkAnchorsAllPages(
	ctx context.Context,
	list func(context.Context, multicloudsdk.ListNetworkAnchorsRequest) (multicloudsdk.ListNetworkAnchorsResponse, error),
	request multicloudsdk.ListNetworkAnchorsRequest,
) (multicloudsdk.ListNetworkAnchorsResponse, error) {
	if list == nil {
		return multicloudsdk.ListNetworkAnchorsResponse{}, fmt.Errorf("NetworkAnchor list operation is not configured")
	}
	if request.Limit == nil {
		request.Limit = common.Int(100)
	}

	var combined multicloudsdk.ListNetworkAnchorsResponse
	seenPages := map[string]struct{}{}
	for {
		response, err := list(ctx, request)
		if err != nil {
			return combined, err
		}
		if combined.OpcRequestId == nil {
			combined.OpcRequestId = response.OpcRequestId
		}
		combined.Items = append(combined.Items, response.Items...)

		nextPage := strings.TrimSpace(stringValue(response.OpcNextPage))
		if nextPage == "" {
			return combined, nil
		}
		if _, exists := seenPages[nextPage]; exists {
			return combined, fmt.Errorf("NetworkAnchor list pagination returned repeated page token %q", nextPage)
		}
		seenPages[nextPage] = struct{}{}
		request.Page = common.String(nextPage)
	}
}

func matchingNetworkAnchorSummaries(target networkAnchorBindTarget, items []multicloudsdk.NetworkAnchorSummary) []multicloudsdk.NetworkAnchorSummary {
	matches := make([]multicloudsdk.NetworkAnchorSummary, 0, len(items))
	for _, item := range items {
		if !networkAnchorSummaryMatches(target, item) {
			continue
		}
		matches = append(matches, item)
	}
	return matches
}

func networkAnchorSummaryMatches(target networkAnchorBindTarget, item multicloudsdk.NetworkAnchorSummary) bool {
	if !networkAnchorRequiredValuesMatch([]networkAnchorRequiredValue{
		{want: target.id, got: stringValue(item.Id)},
		{want: target.displayName, got: stringValue(item.DisplayName)},
		{want: target.compartmentID, got: stringValue(item.CompartmentId)},
		{want: target.ociVCNID, got: stringValue(item.VcnId)},
	}) {
		return false
	}
	if target.subscriptionService != "" && item.SubscriptionType != "" && !strings.EqualFold(string(item.SubscriptionType), target.subscriptionService) {
		return false
	}
	return true
}

type networkAnchorRequiredValue struct {
	want string
	got  string
}

func networkAnchorRequiredValuesMatch(values []networkAnchorRequiredValue) bool {
	for _, value := range values {
		if value.want != "" && value.got != value.want {
			return false
		}
	}
	return true
}

func projectNetworkAnchorFromGet(resource *multicloudv1beta1.NetworkAnchor, response multicloudsdk.GetNetworkAnchorResponse) {
	if resource == nil {
		return
	}
	body := response.NetworkAnchor
	status := &resource.Status
	status.Id = stringValue(body.Id)
	status.DisplayName = stringValue(body.DisplayName)
	status.CompartmentId = stringValue(body.CompartmentId)
	status.ResourceAnchorId = stringValue(body.ResourceAnchorId)
	status.TimeCreated = sdkTimeString(body.TimeCreated)
	status.NetworkAnchorLifecycleState = string(body.NetworkAnchorLifecycleState)
	status.FreeformTags = cloneStringMap(body.FreeformTags)
	status.DefinedTags = convertDefinedTags(body.DefinedTags)
	status.TimeUpdated = sdkTimeString(body.TimeUpdated)
	status.LifecycleDetails = stringValue(body.LifecycleDetails)
	status.SystemTags = convertDefinedTags(body.SystemTags)
	status.SetupMode = string(body.SetupMode)
	status.ClusterPlacementGroupId = stringValue(body.ClusterPlacementGroupId)
	status.OciMetadataItem = projectOciNetworkMetadata(body.OciMetadataItem)
	status.CloudServiceProviderMetadataItem = projectCloudServiceProviderMetadata(body.CloudServiceProviderMetadataItem)
	status.SubscriptionType = string(body.SubscriptionType)
	if status.Id != "" {
		status.OsokStatus.Ocid = shared.OCID(status.Id)
	}
}

func projectNetworkAnchorFromSummary(resource *multicloudv1beta1.NetworkAnchor, summary multicloudsdk.NetworkAnchorSummary) {
	if resource == nil {
		return
	}
	status := &resource.Status
	status.Id = stringValue(summary.Id)
	status.DisplayName = stringValue(summary.DisplayName)
	status.CompartmentId = stringValue(summary.CompartmentId)
	status.ResourceAnchorId = stringValue(summary.ResourceAnchorId)
	status.NetworkAnchorConnectionStatus = string(summary.NetworkAnchorConnectionStatus)
	status.TimeCreated = sdkTimeString(summary.TimeCreated)
	status.NetworkAnchorLifecycleState = string(summary.NetworkAnchorLifecycleState)
	status.FreeformTags = cloneStringMap(summary.FreeformTags)
	status.DefinedTags = convertDefinedTags(summary.DefinedTags)
	status.VcnId = stringValue(summary.VcnId)
	status.VcnName = stringValue(summary.VcnName)
	status.ClusterPlacementGroupId = stringValue(summary.ClusterPlacementGroupId)
	status.TimeUpdated = sdkTimeString(summary.TimeUpdated)
	status.CspAdditionalProperties = cloneStringMap(summary.CspAdditionalProperties)
	status.CspNetworkAnchorId = stringValue(summary.CspNetworkAnchorId)
	status.NetworkAnchorUri = stringValue(summary.NetworkAnchorUri)
	status.LifecycleDetails = stringValue(summary.LifecycleDetails)
	status.SystemTags = convertDefinedTags(summary.SystemTags)
	status.SubscriptionType = string(summary.SubscriptionType)
	if status.Id != "" {
		status.OsokStatus.Ocid = shared.OCID(status.Id)
	}
}

func projectOciNetworkMetadata(source *multicloudsdk.OciNetworkMetadata) multicloudv1beta1.NetworkAnchorOciMetadataItem {
	if source == nil {
		return multicloudv1beta1.NetworkAnchorOciMetadataItem{}
	}
	target := multicloudv1beta1.NetworkAnchorOciMetadataItem{
		NetworkAnchorConnectionStatus:  string(source.NetworkAnchorConnectionStatus),
		Subnets:                        make([]multicloudv1beta1.NetworkAnchorOciMetadataItemSubnet, 0, len(source.Subnets)),
		DnsListeningEndpointIpAddress:  stringValue(source.DnsListeningEndpointIpAddress),
		DnsForwardingEndpointIpAddress: stringValue(source.DnsForwardingEndpointIpAddress),
		DnsForwardingConfig:            cloneStringMapSlice(source.DnsForwardingConfig),
	}
	if source.Vcn != nil {
		target.Vcn = multicloudv1beta1.NetworkAnchorOciMetadataItemVcn{
			VcnId:            stringValue(source.Vcn.VcnId),
			VcnName:          stringValue(source.Vcn.VcnName),
			CidrBlocks:       cloneStringSlice(source.Vcn.CidrBlocks),
			BackupCidrBlocks: cloneStringSlice(source.Vcn.BackupCidrBlocks),
			DnsLabel:         stringValue(source.Vcn.DnsLabel),
		}
	}
	if source.Dns != nil {
		target.Dns = multicloudv1beta1.NetworkAnchorOciMetadataItemDns{
			CustomDomainName: stringValue(source.Dns.CustomDomainName),
		}
	}
	for _, subnet := range source.Subnets {
		target.Subnets = append(target.Subnets, multicloudv1beta1.NetworkAnchorOciMetadataItemSubnet{
			Type:     string(subnet.Type),
			SubnetId: stringValue(subnet.SubnetId),
			Label:    stringValue(subnet.Label),
		})
	}
	return target
}

func projectCloudServiceProviderMetadata(source *multicloudsdk.CloudServiceProviderNetworkMetadataItem) multicloudv1beta1.NetworkAnchorCloudServiceProviderMetadataItem {
	if source == nil {
		return multicloudv1beta1.NetworkAnchorCloudServiceProviderMetadataItem{}
	}
	return multicloudv1beta1.NetworkAnchorCloudServiceProviderMetadataItem{
		Region:                  stringValue(source.Region),
		OdbNetworkId:            stringValue(source.OdbNetworkId),
		CidrBlocks:              cloneStringSlice(source.CidrBlocks),
		NetworkAnchorUri:        stringValue(source.NetworkAnchorUri),
		CspAdditionalProperties: cloneStringMap(source.CspAdditionalProperties),
		DnsForwardingConfig:     cloneStringMapSlice(source.DnsForwardingConfig),
	}
}

func networkAnchorLifecycleProjection(lifecycle string) (shared.OSOKConditionType, shared.OSOKAsyncPhase, shared.OSOKAsyncNormalizedClass, bool) {
	switch multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateEnum(strings.ToUpper(strings.TrimSpace(lifecycle))) {
	case multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateActive:
		return shared.Active, "", shared.OSOKAsyncClassSucceeded, false
	case multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateCreating:
		return shared.Provisioning, shared.OSOKAsyncPhaseCreate, shared.OSOKAsyncClassPending, true
	case multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateUpdating:
		return shared.Updating, shared.OSOKAsyncPhaseUpdate, shared.OSOKAsyncClassPending, true
	case multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateDeleting:
		return shared.Terminating, shared.OSOKAsyncPhaseDelete, shared.OSOKAsyncClassPending, true
	case multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateDeleted:
		return shared.Failed, "", shared.OSOKAsyncClassFailed, false
	case multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateFailed:
		return shared.Failed, "", shared.OSOKAsyncClassFailed, false
	default:
		return shared.Failed, "", shared.OSOKAsyncClassUnknown, false
	}
}

func networkAnchorLifecycleMessage(condition shared.OSOKConditionType, lifecycle string, details string) string {
	if details = strings.TrimSpace(details); details != "" {
		return details
	}
	state := strings.TrimSpace(lifecycle)
	if state == "" {
		return "NetworkAnchor lifecycle state is unknown"
	}
	if condition == shared.Active {
		return "NetworkAnchor is ACTIVE"
	}
	return fmt.Sprintf("NetworkAnchor lifecycle state is %s", state)
}

func networkAnchorAnnotation(resource *multicloudv1beta1.NetworkAnchor, key string) string {
	if resource == nil || len(resource.Annotations) == 0 {
		return ""
	}
	return strings.TrimSpace(resource.Annotations[key])
}

func networkAnchorBoolAnnotation(resource *multicloudv1beta1.NetworkAnchor, key string) (*bool, error) {
	value := strings.ToLower(networkAnchorAnnotation(resource, key))
	switch value {
	case "":
		return nil, nil
	case "true":
		return common.Bool(true), nil
	case "false":
		return common.Bool(false), nil
	default:
		return nil, fmt.Errorf("NetworkAnchor annotation %q must be true or false", key)
	}
}

func firstNetworkAnchorSubnetID(resource *multicloudv1beta1.NetworkAnchor) string {
	if resource == nil {
		return ""
	}
	for _, subnet := range resource.Status.OciMetadataItem.Subnets {
		if subnetID := strings.TrimSpace(subnet.SubnetId); subnetID != "" {
			return subnetID
		}
	}
	return ""
}

func firstNonEmptyNetworkAnchorString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func stringPtrOrNil(value string) *string {
	if value = strings.TrimSpace(value); value != "" {
		return common.String(value)
	}
	return nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func sdkTimeString(value *common.SDKTime) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func cloneStringSlice(source []string) []string {
	if len(source) == 0 {
		return nil
	}
	return append([]string(nil), source...)
}

func cloneStringMapSlice(source []map[string]string) []map[string]string {
	if len(source) == 0 {
		return nil
	}
	target := make([]map[string]string, 0, len(source))
	for _, item := range source {
		target = append(target, cloneStringMap(item))
	}
	return target
}

func convertDefinedTags(source map[string]map[string]interface{}) map[string]shared.MapValue {
	if len(source) == 0 {
		return nil
	}
	target := make(map[string]shared.MapValue, len(source))
	for namespace, values := range source {
		if len(values) == 0 {
			continue
		}
		converted := make(shared.MapValue, len(values))
		for key, value := range values {
			converted[key] = fmt.Sprint(value)
		}
		target[namespace] = converted
	}
	if len(target) == 0 {
		return nil
	}
	return target
}

func networkAnchorOrdinaryNotFound(err error) bool {
	serviceErr, ok := networkAnchorServiceError(err)
	return ok && serviceErr.GetHTTPStatusCode() == 404 && strings.EqualFold(serviceErr.GetCode(), "NotFound")
}

func networkAnchorAuthShapedNotFound(err error) bool {
	serviceErr, ok := networkAnchorServiceError(err)
	return ok && serviceErr.GetHTTPStatusCode() == 404 && strings.EqualFold(serviceErr.GetCode(), "NotAuthorizedOrNotFound")
}

func networkAnchorServiceError(err error) (common.ServiceError, bool) {
	var serviceErr common.ServiceError
	if errors.As(err, &serviceErr) {
		return serviceErr, true
	}
	return nil, false
}
