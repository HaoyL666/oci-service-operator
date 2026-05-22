/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package multicloudmetadata

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
	MultiCloudMetadataCompartmentIDAnnotation  = "multicloud.oracle.com/compartment-id"
	MultiCloudMetadataSubscriptionIDAnnotation = "multicloud.oracle.com/subscription-id"
)

type multiCloudMetadataOCIClient interface {
	GetMultiCloudMetadata(context.Context, multicloudsdk.GetMultiCloudMetadataRequest) (multicloudsdk.GetMultiCloudMetadataResponse, error)
	ListMultiCloudMetadata(context.Context, multicloudsdk.ListMultiCloudMetadataRequest) (multicloudsdk.ListMultiCloudMetadataResponse, error)
}

type multiCloudMetadataReadOnlyClient struct {
	client   multiCloudMetadataOCIClient
	provider common.ConfigurationProvider
	log      loggerutil.OSOKLogger
	initErr  error
}

type multiCloudMetadataIdentity struct {
	compartmentID  string
	subscriptionID string
}

type multiCloudMetadataIdentitySources struct {
	annotationCompartmentID  string
	annotationSubscriptionID string
	recordedCompartmentID    string
	recordedSubscriptionID   string
	recordedOCID             string
}

func init() {
	newMultiCloudMetadataServiceClient = func(manager *MultiCloudMetadataServiceManager) MultiCloudMetadataServiceClient {
		if manager == nil {
			return newMultiCloudMetadataServiceClientWithOCIClient(loggerutil.OSOKLogger{}, nil, fmt.Errorf("MultiCloudMetadata service manager is nil"))
		}
		sdkClient, err := multicloudsdk.NewMultiCloudsMetadataClientWithConfigurationProvider(manager.Provider)
		return newMultiCloudMetadataServiceClientWithProvider(manager.Log, sdkClient, manager.Provider, err)
	}
}

func newMultiCloudMetadataServiceClientWithOCIClient(
	log loggerutil.OSOKLogger,
	client multiCloudMetadataOCIClient,
	initErr error,
) MultiCloudMetadataServiceClient {
	return newMultiCloudMetadataServiceClientWithProvider(log, client, nil, initErr)
}

func newMultiCloudMetadataServiceClientWithProvider(
	log loggerutil.OSOKLogger,
	client multiCloudMetadataOCIClient,
	provider common.ConfigurationProvider,
	initErr error,
) MultiCloudMetadataServiceClient {
	return &multiCloudMetadataReadOnlyClient{
		client:   client,
		provider: provider,
		log:      log,
		initErr:  initErr,
	}
}

func (c *multiCloudMetadataReadOnlyClient) CreateOrUpdate(
	ctx context.Context,
	resource *multicloudv1beta1.MultiCloudMetadata,
	_ ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	if c.initErr != nil {
		return c.fail(resource, fmt.Errorf("initialize MultiCloudMetadata OCI client: %w", c.initErr))
	}
	if c.client == nil {
		return c.fail(resource, fmt.Errorf("MultiCloudMetadata OCI client is nil"))
	}

	identity, err := resolveMultiCloudMetadataIdentity(resource, c.provider)
	if err != nil {
		return c.fail(resource, err)
	}
	if identity.subscriptionID != "" {
		return c.observeByGet(ctx, resource, identity)
	}
	return c.observeByList(ctx, resource, identity)
}

func (c *multiCloudMetadataReadOnlyClient) Delete(_ context.Context, resource *multicloudv1beta1.MultiCloudMetadata) (bool, error) {
	markMultiCloudMetadataDeleted(resource, "OCI delete is not supported for read-only MultiCloudMetadata")
	return true, nil
}

func (c *multiCloudMetadataReadOnlyClient) observeByGet(
	ctx context.Context,
	resource *multicloudv1beta1.MultiCloudMetadata,
	identity multiCloudMetadataIdentity,
) (servicemanager.OSOKResponse, error) {
	response, err := c.client.GetMultiCloudMetadata(ctx, multicloudsdk.GetMultiCloudMetadataRequest{
		CompartmentId:  common.String(identity.compartmentID),
		SubscriptionId: common.String(identity.subscriptionID),
	})
	if err != nil {
		return c.fail(resource, err)
	}

	projectMultiCloudMetadata(resource, identity, response.MultiCloudMetadata)
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)
	return markMultiCloudMetadataActive(resource, c.log), nil
}

func (c *multiCloudMetadataReadOnlyClient) observeByList(
	ctx context.Context,
	resource *multicloudv1beta1.MultiCloudMetadata,
	identity multiCloudMetadataIdentity,
) (servicemanager.OSOKResponse, error) {
	items, response, err := c.listMultiCloudMetadataAllPages(ctx, identity.compartmentID)
	if err != nil {
		return c.fail(resource, err)
	}
	item, err := selectMultiCloudMetadataSummary(items, identity)
	if err != nil {
		return c.fail(resource, err)
	}

	projectMultiCloudMetadataSummary(resource, identity, item)
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)
	return markMultiCloudMetadataActive(resource, c.log), nil
}

func (c *multiCloudMetadataReadOnlyClient) listMultiCloudMetadataAllPages(
	ctx context.Context,
	compartmentID string,
) ([]multicloudsdk.MultiCloudMetadataSummary, multicloudsdk.ListMultiCloudMetadataResponse, error) {
	request := multicloudsdk.ListMultiCloudMetadataRequest{
		CompartmentId: common.String(compartmentID),
	}
	seenPages := map[string]struct{}{}
	var combined multicloudsdk.ListMultiCloudMetadataResponse

	for {
		response, err := c.client.ListMultiCloudMetadata(ctx, request)
		if err != nil {
			return nil, multicloudsdk.ListMultiCloudMetadataResponse{}, err
		}
		combined.RawResponse = response.RawResponse
		combined.OpcRequestId = response.OpcRequestId
		combined.Items = append(combined.Items, response.Items...)

		nextPage := strings.TrimSpace(stringValue(response.OpcNextPage))
		if nextPage == "" {
			combined.OpcNextPage = nil
			return combined.Items, combined, nil
		}
		if _, ok := seenPages[nextPage]; ok {
			return nil, multicloudsdk.ListMultiCloudMetadataResponse{}, fmt.Errorf("MultiCloudMetadata list pagination repeated page token %q", nextPage)
		}
		seenPages[nextPage] = struct{}{}
		request.Page = common.String(nextPage)
		combined.OpcNextPage = response.OpcNextPage
	}
}

func resolveMultiCloudMetadataIdentity(
	resource *multicloudv1beta1.MultiCloudMetadata,
	provider common.ConfigurationProvider,
) (multiCloudMetadataIdentity, error) {
	if resource == nil {
		return multiCloudMetadataIdentity{}, fmt.Errorf("MultiCloudMetadata resource is nil")
	}

	sources := multiCloudMetadataIdentitySourcesFor(resource)
	if err := validateMultiCloudMetadataIdentitySources(sources); err != nil {
		return multiCloudMetadataIdentity{}, err
	}

	compartmentID, err := resolveMultiCloudMetadataCompartmentID(sources, provider)
	if err != nil {
		return multiCloudMetadataIdentity{}, err
	}
	return multiCloudMetadataIdentity{
		compartmentID:  compartmentID,
		subscriptionID: firstNonEmpty(sources.annotationSubscriptionID, sources.recordedSubscriptionID, sources.recordedOCID),
	}, nil
}

func resolveMultiCloudMetadataCompartmentID(
	sources multiCloudMetadataIdentitySources,
	provider common.ConfigurationProvider,
) (string, error) {
	if compartmentID := firstNonEmpty(sources.annotationCompartmentID, sources.recordedCompartmentID); compartmentID != "" {
		return compartmentID, nil
	}

	if provider == nil {
		return "", fmt.Errorf(
			"MultiCloudMetadata requires %s annotation, status.compartmentId, or a configured OCI provider tenancy OCID to read OCI metadata",
			MultiCloudMetadataCompartmentIDAnnotation,
		)
	}
	tenancyID, err := provider.TenancyOCID()
	if err != nil {
		return "", fmt.Errorf(
			"MultiCloudMetadata requires %s annotation or status.compartmentId because provider tenancy OCID could not be resolved: %w",
			MultiCloudMetadataCompartmentIDAnnotation,
			err,
		)
	}
	if tenancyID = strings.TrimSpace(tenancyID); tenancyID != "" {
		return tenancyID, nil
	}
	return "", fmt.Errorf(
		"MultiCloudMetadata requires %s annotation or status.compartmentId because provider tenancy OCID is empty",
		MultiCloudMetadataCompartmentIDAnnotation,
	)
}

func multiCloudMetadataIdentitySourcesFor(resource *multicloudv1beta1.MultiCloudMetadata) multiCloudMetadataIdentitySources {
	return multiCloudMetadataIdentitySources{
		annotationCompartmentID:  annotationValue(resource, MultiCloudMetadataCompartmentIDAnnotation),
		annotationSubscriptionID: annotationValue(resource, MultiCloudMetadataSubscriptionIDAnnotation),
		recordedCompartmentID:    strings.TrimSpace(resource.Status.CompartmentId),
		recordedSubscriptionID:   strings.TrimSpace(resource.Status.SubscriptionId),
		recordedOCID:             strings.TrimSpace(string(resource.Status.OsokStatus.Ocid)),
	}
}

func validateMultiCloudMetadataIdentitySources(sources multiCloudMetadataIdentitySources) error {
	if err := validateRecordedSubscriptionIdentity(sources); err != nil {
		return err
	}
	if err := validateCompartmentIdentity(sources); err != nil {
		return err
	}
	return validateAnnotationSubscriptionIdentity(sources)
}

func validateRecordedSubscriptionIdentity(sources multiCloudMetadataIdentitySources) error {
	if sources.recordedSubscriptionID == "" || sources.recordedOCID == "" || sources.recordedSubscriptionID == sources.recordedOCID {
		return nil
	}
	return fmt.Errorf(
		"MultiCloudMetadata recorded identity drift: status.subscriptionId %q does not match status.status.ocid %q",
		sources.recordedSubscriptionID,
		sources.recordedOCID,
	)
}

func validateCompartmentIdentity(sources multiCloudMetadataIdentitySources) error {
	if sources.annotationCompartmentID == "" || sources.recordedCompartmentID == "" || sources.annotationCompartmentID == sources.recordedCompartmentID {
		return nil
	}
	return fmt.Errorf(
		"MultiCloudMetadata immutable identity drift: %s %q does not match status.compartmentId %q",
		MultiCloudMetadataCompartmentIDAnnotation,
		sources.annotationCompartmentID,
		sources.recordedCompartmentID,
	)
}

func validateAnnotationSubscriptionIdentity(sources multiCloudMetadataIdentitySources) error {
	if sources.annotationSubscriptionID != "" && sources.recordedSubscriptionID != "" && sources.annotationSubscriptionID != sources.recordedSubscriptionID {
		return fmt.Errorf(
			"MultiCloudMetadata immutable identity drift: %s %q does not match status.subscriptionId %q",
			MultiCloudMetadataSubscriptionIDAnnotation,
			sources.annotationSubscriptionID,
			sources.recordedSubscriptionID,
		)
	}
	if sources.annotationSubscriptionID != "" && sources.recordedOCID != "" && sources.annotationSubscriptionID != sources.recordedOCID {
		return fmt.Errorf(
			"MultiCloudMetadata immutable identity drift: %s %q does not match status.status.ocid %q",
			MultiCloudMetadataSubscriptionIDAnnotation,
			sources.annotationSubscriptionID,
			sources.recordedOCID,
		)
	}
	return nil
}

func selectMultiCloudMetadataSummary(
	items []multicloudsdk.MultiCloudMetadataSummary,
	identity multiCloudMetadataIdentity,
) (multicloudsdk.MultiCloudMetadataSummary, error) {
	var matches []multicloudsdk.MultiCloudMetadataSummary
	for _, item := range items {
		if stringValue(item.CompartmentId) != identity.compartmentID {
			continue
		}
		if identity.subscriptionID != "" && stringValue(item.SubscriptionId) != identity.subscriptionID {
			continue
		}
		matches = append(matches, item)
	}

	switch len(matches) {
	case 0:
		return multicloudsdk.MultiCloudMetadataSummary{}, fmt.Errorf("MultiCloudMetadata list found no metadata for compartmentId %q", identity.compartmentID)
	case 1:
		return matches[0], nil
	default:
		return multicloudsdk.MultiCloudMetadataSummary{}, fmt.Errorf(
			"MultiCloudMetadata list found %d matches for compartmentId %q; set %s to bind one subscription",
			len(matches),
			identity.compartmentID,
			MultiCloudMetadataSubscriptionIDAnnotation,
		)
	}
}

func projectMultiCloudMetadata(
	resource *multicloudv1beta1.MultiCloudMetadata,
	identity multiCloudMetadataIdentity,
	metadata multicloudsdk.MultiCloudMetadata,
) {
	if resource == nil {
		return
	}
	compartmentID := firstNonEmpty(stringValue(metadata.CompartmentId), identity.compartmentID)
	subscriptionID := firstNonEmpty(stringValue(metadata.SubscriptionId), identity.subscriptionID)
	resource.Status.CompartmentId = compartmentID
	resource.Status.SubscriptionId = subscriptionID
	resource.Status.TimeCreated = sdkTimeString(metadata.TimeCreated)
	resource.Status.FreeformTags = copyStringMap(metadata.FreeformTags)
	resource.Status.DefinedTags = mapValueTags(metadata.DefinedTags)
	resource.Status.SystemTags = mapValueTags(metadata.SystemTags)
	if subscriptionID != "" {
		resource.Status.OsokStatus.Ocid = shared.OCID(subscriptionID)
	}
}

func projectMultiCloudMetadataSummary(
	resource *multicloudv1beta1.MultiCloudMetadata,
	identity multiCloudMetadataIdentity,
	metadata multicloudsdk.MultiCloudMetadataSummary,
) {
	if resource == nil {
		return
	}
	compartmentID := firstNonEmpty(stringValue(metadata.CompartmentId), identity.compartmentID)
	subscriptionID := firstNonEmpty(stringValue(metadata.SubscriptionId), identity.subscriptionID)
	resource.Status.CompartmentId = compartmentID
	resource.Status.SubscriptionId = subscriptionID
	resource.Status.TimeCreated = sdkTimeString(metadata.TimeCreated)
	resource.Status.FreeformTags = copyStringMap(metadata.FreeformTags)
	resource.Status.DefinedTags = mapValueTags(metadata.DefinedTags)
	resource.Status.SystemTags = mapValueTags(metadata.SystemTags)
	if subscriptionID != "" {
		resource.Status.OsokStatus.Ocid = shared.OCID(subscriptionID)
	}
}

func markMultiCloudMetadataActive(
	resource *multicloudv1beta1.MultiCloudMetadata,
	log loggerutil.OSOKLogger,
) servicemanager.OSOKResponse {
	if resource == nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}
	}
	status := &resource.Status.OsokStatus
	now := metav1.Now()
	if status.CreatedAt == nil {
		status.CreatedAt = &now
	}
	status.UpdatedAt = &now
	status.Reason = string(shared.Active)
	status.Message = "MultiCloudMetadata observed"
	status.Async.Current = nil
	resource.Status.OsokStatus = util.UpdateOSOKStatusCondition(
		resource.Status.OsokStatus,
		shared.Active,
		corev1.ConditionTrue,
		"",
		status.Message,
		log,
	)
	return servicemanager.OSOKResponse{IsSuccessful: true}
}

func markMultiCloudMetadataDeleted(resource *multicloudv1beta1.MultiCloudMetadata, message string) {
	if resource == nil {
		return
	}
	status := &resource.Status.OsokStatus
	now := metav1.Now()
	status.DeletedAt = &now
	status.UpdatedAt = &now
	status.Reason = string(shared.Terminating)
	status.Message = message
	status.Async.Current = nil
	resource.Status.OsokStatus = util.UpdateOSOKStatusCondition(
		resource.Status.OsokStatus,
		shared.Terminating,
		corev1.ConditionTrue,
		"",
		message,
		loggerutil.OSOKLogger{},
	)
}

func (c *multiCloudMetadataReadOnlyClient) fail(
	resource *multicloudv1beta1.MultiCloudMetadata,
	err error,
) (servicemanager.OSOKResponse, error) {
	if resource != nil && err != nil {
		status := &resource.Status.OsokStatus
		servicemanager.RecordErrorOpcRequestID(status, err)
		now := metav1.Now()
		status.UpdatedAt = &now
		status.Reason = string(shared.Failed)
		status.Message = err.Error()
		resource.Status.OsokStatus = util.UpdateOSOKStatusCondition(
			resource.Status.OsokStatus,
			shared.Failed,
			corev1.ConditionFalse,
			"",
			err.Error(),
			c.log,
		)
	}
	return servicemanager.OSOKResponse{IsSuccessful: false}, err
}

func annotationValue(resource *multicloudv1beta1.MultiCloudMetadata, key string) string {
	if resource == nil {
		return ""
	}
	return strings.TrimSpace(resource.GetAnnotations()[key])
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func mapValueTags(values map[string]map[string]interface{}) map[string]shared.MapValue {
	if len(values) == 0 {
		return nil
	}
	converted := make(map[string]shared.MapValue, len(values))
	for namespace, tags := range values {
		if len(tags) == 0 {
			converted[namespace] = shared.MapValue{}
			continue
		}
		convertedTags := make(shared.MapValue, len(tags))
		for key, value := range tags {
			if value == nil {
				continue
			}
			switch typed := value.(type) {
			case string:
				convertedTags[key] = typed
			case fmt.Stringer:
				convertedTags[key] = typed.String()
			default:
				convertedTags[key] = fmt.Sprint(typed)
			}
		}
		converted[namespace] = convertedTags
	}
	return converted
}

func sdkTimeString(value *common.SDKTime) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
