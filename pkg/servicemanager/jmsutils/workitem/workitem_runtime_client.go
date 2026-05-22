/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package workitem

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	jmsutilssdk "github.com/oracle/oci-go-sdk/v65/jmsutils"
	jmsutilsv1beta1 "github.com/oracle/oci-service-operator/api/jmsutils/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	"github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	workItemWorkRequestIDAnnotation       = "jmsutils.oracle.com/work-request-id"
	workItemIDAnnotation                  = "jmsutils.oracle.com/work-item-id"
	workItemLegacyWorkRequestIDAnnotation = "jmsutils.oracle.com/workRequestId"
	workItemLegacyIDAnnotation            = "jmsutils.oracle.com/workItemId"

	workItemListLimit       = 100
	workItemRequeueDuration = time.Minute
)

type workItemOCIClient interface {
	ListWorkItems(context.Context, jmsutilssdk.ListWorkItemsRequest) (jmsutilssdk.ListWorkItemsResponse, error)
}

type workItemRuntimeClient struct {
	client  workItemOCIClient
	log     loggerutil.OSOKLogger
	initErr error
}

type workItemIdentity struct {
	workRequestID string
	itemID        string
}

func init() {
	registerWorkItemRuntimeHooksMutator(func(manager *WorkItemServiceManager, hooks *WorkItemRuntimeHooks) {
		client, err := newWorkItemRuntimeOCIClient(manager)
		applyWorkItemRuntimeHooks(manager, hooks, client, err)
	})
}

func newWorkItemRuntimeOCIClient(manager *WorkItemServiceManager) (workItemOCIClient, error) {
	if manager == nil {
		return nil, fmt.Errorf("WorkItem service manager is nil")
	}
	client, err := jmsutilssdk.NewJmsUtilsClientWithConfigurationProvider(manager.Provider)
	if err != nil {
		return nil, fmt.Errorf("initialize WorkItem OCI client: %w", err)
	}
	return client, nil
}

func applyWorkItemRuntimeHooks(
	manager *WorkItemServiceManager,
	hooks *WorkItemRuntimeHooks,
	client workItemOCIClient,
	initErr error,
) {
	if hooks == nil {
		return
	}
	hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(delegate WorkItemServiceClient) WorkItemServiceClient {
		log := loggerutil.OSOKLogger{}
		if manager != nil {
			log = manager.Log
		}
		return newWorkItemRuntimeClient(delegate, client, log, initErr)
	})
}

func newWorkItemRuntimeClient(
	_ WorkItemServiceClient,
	client workItemOCIClient,
	log loggerutil.OSOKLogger,
	initErr error,
) *workItemRuntimeClient {
	return &workItemRuntimeClient{
		client:  client,
		log:     log,
		initErr: initErr,
	}
}

func newWorkItemServiceClientWithOCIClient(
	log loggerutil.OSOKLogger,
	client workItemOCIClient,
) WorkItemServiceClient {
	return newWorkItemRuntimeClient(nil, client, log, nil)
}

func (c *workItemRuntimeClient) CreateOrUpdate(
	ctx context.Context,
	resource *jmsutilsv1beta1.WorkItem,
	_ ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	if err := c.ensureClient(); err != nil {
		return c.fail(resource, err)
	}
	identity, err := resolveWorkItemIdentity(resource)
	if err != nil {
		return c.fail(resource, err)
	}
	if err := validateTrackedWorkItemIdentity(resource, identity); err != nil {
		return c.fail(resource, err)
	}

	items, response, err := c.listAllWorkItems(ctx, identity.workRequestID)
	if err != nil {
		return c.fail(resource, err)
	}
	item, err := selectWorkItem(items, identity)
	if err != nil {
		return c.fail(resource, err)
	}

	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)
	return c.projectWorkItem(resource, item), nil
}

func (c *workItemRuntimeClient) Delete(
	_ context.Context,
	resource *jmsutilsv1beta1.WorkItem,
) (bool, error) {
	if resource == nil {
		return false, fmt.Errorf("WorkItem resource is nil")
	}
	message := "OCI WorkItem delete is not supported by the JMS Utils SDK; removing the observer finalizer without mutating OCI"
	markWorkItemDeleted(resource, message, c.log)
	return true, nil
}

func (c *workItemRuntimeClient) ensureClient() error {
	if c.initErr != nil {
		return c.initErr
	}
	if c.client == nil {
		return fmt.Errorf("WorkItem OCI client is not configured")
	}
	return nil
}

func (c *workItemRuntimeClient) listAllWorkItems(
	ctx context.Context,
	workRequestID string,
) ([]jmsutilssdk.WorkItemSummary, jmsutilssdk.ListWorkItemsResponse, error) {
	var (
		items    []jmsutilssdk.WorkItemSummary
		response jmsutilssdk.ListWorkItemsResponse
		page     string
	)
	for {
		request := jmsutilssdk.ListWorkItemsRequest{
			WorkRequestId: common.String(workRequestID),
			Limit:         common.Int(workItemListLimit),
		}
		if strings.TrimSpace(page) != "" {
			request.Page = common.String(page)
		}

		var err error
		response, err = c.client.ListWorkItems(ctx, request)
		if err != nil {
			return nil, response, err
		}
		items = append(items, response.Items...)

		page = strings.TrimSpace(stringValue(response.OpcNextPage))
		if page == "" {
			return items, response, nil
		}
	}
}

func (c *workItemRuntimeClient) projectWorkItem(
	resource *jmsutilsv1beta1.WorkItem,
	item jmsutilssdk.WorkItemSummary,
) servicemanager.OSOKResponse {
	id := strings.TrimSpace(stringValue(item.Id))
	workRequestID := strings.TrimSpace(stringValue(item.WorkRequestId))
	rawStatus := strings.TrimSpace(string(item.Status))
	message := workItemStatusMessage(id, rawStatus)
	now := metav1.Now()

	resource.Status.Id = id
	resource.Status.WorkRequestId = workRequestID
	resource.Status.Status = rawStatus
	resource.Status.RetryCount = intValue(item.RetryCount)
	resource.Status.TimeLastUpdated = sdkTimeString(item.TimeLastUpdated)
	resource.Status.Details = workItemDetailsFromSDK(item.Details)

	status := &resource.Status.OsokStatus
	if id != "" {
		status.Ocid = shared.OCID(id)
		if status.CreatedAt == nil {
			status.CreatedAt = &now
		}
	}
	status.UpdatedAt = &now

	class, ok := workItemAsyncClass(item.Status)
	if ok && class != shared.OSOKAsyncClassSucceeded {
		projection := servicemanager.ApplyAsyncOperation(status, &shared.OSOKAsyncOperation{
			Source:          shared.OSOKAsyncSourceLifecycle,
			Phase:           shared.OSOKAsyncPhaseCreate,
			WorkRequestID:   workRequestID,
			RawStatus:       rawStatus,
			NormalizedClass: class,
			Message:         message,
			UpdatedAt:       &now,
		}, c.log)
		return servicemanager.OSOKResponse{
			IsSuccessful:    projection.Condition != shared.Failed,
			ShouldRequeue:   projection.ShouldRequeue,
			RequeueDuration: workItemRequeueDuration,
		}
	}

	servicemanager.ClearAsyncOperation(status)
	condition := shared.Active
	status.Message = message
	status.Reason = string(condition)
	*status = util.UpdateOSOKStatusCondition(*status, condition, corev1.ConditionTrue, "", message, c.log)
	return servicemanager.OSOKResponse{IsSuccessful: true}
}

func (c *workItemRuntimeClient) fail(
	resource *jmsutilsv1beta1.WorkItem,
	err error,
) (servicemanager.OSOKResponse, error) {
	if resource == nil || err == nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	status := &resource.Status.OsokStatus
	servicemanager.RecordErrorOpcRequestID(status, err)
	now := metav1.Now()
	status.UpdatedAt = &now
	status.Message = err.Error()
	status.Reason = string(shared.Failed)
	if status.Async.Current != nil {
		current := *status.Async.Current
		current.NormalizedClass = shared.OSOKAsyncClassFailed
		current.Message = err.Error()
		current.UpdatedAt = &now
		_ = servicemanager.ApplyAsyncOperation(status, &current, c.log)
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	*status = util.UpdateOSOKStatusCondition(*status, shared.Failed, corev1.ConditionFalse, "", err.Error(), c.log)
	return servicemanager.OSOKResponse{IsSuccessful: false}, err
}

func resolveWorkItemIdentity(resource *jmsutilsv1beta1.WorkItem) (workItemIdentity, error) {
	if resource == nil {
		return workItemIdentity{}, fmt.Errorf("WorkItem resource is nil")
	}
	identity := workItemIdentity{
		workRequestID: firstNonEmpty(
			annotationValue(resource, workItemWorkRequestIDAnnotation, workItemLegacyWorkRequestIDAnnotation),
			resource.Status.WorkRequestId,
			currentAsyncWorkRequestID(resource),
		),
		itemID: firstNonEmpty(
			annotationValue(resource, workItemIDAnnotation, workItemLegacyIDAnnotation),
			resource.Status.Id,
			string(resource.Status.OsokStatus.Ocid),
		),
	}
	if identity.workRequestID == "" {
		return workItemIdentity{}, fmt.Errorf(
			"WorkItem requires %s because the JMS Utils SDK only exposes ListWorkItems under a work request",
			workItemWorkRequestIDAnnotation,
		)
	}
	return identity, nil
}

func validateTrackedWorkItemIdentity(resource *jmsutilsv1beta1.WorkItem, desired workItemIdentity) error {
	if resource == nil {
		return fmt.Errorf("WorkItem resource is nil")
	}
	if tracked := strings.TrimSpace(resource.Status.WorkRequestId); tracked != "" && desired.workRequestID != "" && tracked != desired.workRequestID {
		return fmt.Errorf("WorkItem workRequestId is immutable: tracked %q, desired %q", tracked, desired.workRequestID)
	}
	trackedItemID := firstNonEmpty(resource.Status.Id, string(resource.Status.OsokStatus.Ocid))
	if trackedItemID != "" && desired.itemID != "" && trackedItemID != desired.itemID {
		return fmt.Errorf("WorkItem id is immutable: tracked %q, desired %q", trackedItemID, desired.itemID)
	}
	return nil
}

func selectWorkItem(items []jmsutilssdk.WorkItemSummary, identity workItemIdentity) (jmsutilssdk.WorkItemSummary, error) {
	if identity.itemID != "" {
		var matches []jmsutilssdk.WorkItemSummary
		for _, item := range items {
			if strings.TrimSpace(stringValue(item.Id)) == identity.itemID {
				matches = append(matches, item)
			}
		}
		switch len(matches) {
		case 1:
			return matches[0], nil
		case 0:
			return jmsutilssdk.WorkItemSummary{}, fmt.Errorf("WorkItem %q was not found under work request %q", identity.itemID, identity.workRequestID)
		default:
			return jmsutilssdk.WorkItemSummary{}, fmt.Errorf("work request %q returned multiple WorkItems with id %q", identity.workRequestID, identity.itemID)
		}
	}

	switch len(items) {
	case 1:
		return items[0], nil
	case 0:
		return jmsutilssdk.WorkItemSummary{}, fmt.Errorf("work request %q returned no WorkItems", identity.workRequestID)
	default:
		return jmsutilssdk.WorkItemSummary{}, fmt.Errorf("work request %q returned %d WorkItems; set %s to choose one", identity.workRequestID, len(items), workItemIDAnnotation)
	}
}

func workItemDetailsFromSDK(details jmsutilssdk.WorkItemDetails) jmsutilsv1beta1.WorkItemDetails {
	if details == nil {
		return jmsutilsv1beta1.WorkItemDetails{}
	}

	out := jmsutilsv1beta1.WorkItemDetails{
		WorkItemType: strings.TrimSpace(string(details.GetWorkItemType())),
	}
	if raw, err := json.Marshal(details); err == nil {
		out.JsonData = string(raw)
		var generic struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(raw, &generic); err == nil {
			out.Kind = strings.TrimSpace(generic.Kind)
		}
	}

	switch typed := details.(type) {
	case jmsutilssdk.JavaMigrationWorkItemDetails:
		out.Kind = string(jmsutilssdk.WorkItemDetailsKindJavaMigration)
		out.TargetJdkVersion = stringValue(typed.TargetJdkVersion)
		out.InputApplicationsObjectStoragePaths = stringValue(typed.InputApplicationsObjectStoragePaths)
		out.AnalysisProjectName = stringValue(typed.AnalysisProjectName)
	case jmsutilssdk.PerformanceTuningWorkItemDetails:
		out.Kind = string(jmsutilssdk.WorkItemDetailsKindPerformanceTuning)
		out.ArtifactObjectStoragePath = stringValue(typed.ArtifactObjectStoragePath)
		out.AnalysisProjectName = stringValue(typed.AnalysisProjectName)
	case jmsutilssdk.BasicWorkItemDetails:
		out.Kind = string(jmsutilssdk.WorkItemDetailsKindBasic)
	}

	return out
}

func workItemAsyncClass(status jmsutilssdk.WorkItemStatusEnum) (shared.OSOKAsyncNormalizedClass, bool) {
	switch status {
	case jmsutilssdk.WorkItemStatusAccepted,
		jmsutilssdk.WorkItemStatusInProgress,
		jmsutilssdk.WorkItemStatusCanceling,
		jmsutilssdk.WorkItemStatusRetrying:
		return shared.OSOKAsyncClassPending, true
	case jmsutilssdk.WorkItemStatusSucceeded,
		jmsutilssdk.WorkItemStatusSkipped:
		return shared.OSOKAsyncClassSucceeded, true
	case jmsutilssdk.WorkItemStatusCanceled:
		return shared.OSOKAsyncClassCanceled, true
	case jmsutilssdk.WorkItemStatusNeedsAttention:
		return shared.OSOKAsyncClassAttention, true
	default:
		return shared.OSOKAsyncClassUnknown, status != ""
	}
}

func markWorkItemDeleted(resource *jmsutilsv1beta1.WorkItem, message string, log loggerutil.OSOKLogger) {
	now := metav1.Now()
	status := &resource.Status.OsokStatus
	status.DeletedAt = &now
	status.UpdatedAt = &now
	status.Message = message
	status.Reason = string(shared.Terminating)
	servicemanager.ClearAsyncOperation(status)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Terminating, corev1.ConditionTrue, "", message, log)
}

func workItemStatusMessage(id string, status string) string {
	status = strings.TrimSpace(status)
	id = strings.TrimSpace(id)
	if id == "" {
		if status == "" {
			return "OCI WorkItem observed"
		}
		return fmt.Sprintf("OCI WorkItem is %s", status)
	}
	if status == "" {
		return fmt.Sprintf("OCI WorkItem %s observed", id)
	}
	return fmt.Sprintf("OCI WorkItem %s is %s", id, status)
}

func currentAsyncWorkRequestID(resource *jmsutilsv1beta1.WorkItem) string {
	if resource == nil || resource.Status.OsokStatus.Async.Current == nil {
		return ""
	}
	return strings.TrimSpace(resource.Status.OsokStatus.Async.Current.WorkRequestID)
}

func annotationValue(resource *jmsutilsv1beta1.WorkItem, keys ...string) string {
	if resource == nil {
		return ""
	}
	for _, key := range keys {
		if value := strings.TrimSpace(resource.Annotations[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value := strings.TrimSpace(value); value != "" {
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

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func sdkTimeString(value *common.SDKTime) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
