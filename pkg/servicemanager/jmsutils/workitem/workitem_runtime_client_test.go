/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package workitem

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	jmsutilssdk "github.com/oracle/oci-go-sdk/v65/jmsutils"
	jmsutilsv1beta1 "github.com/oracle/oci-service-operator/api/jmsutils/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil"
	"github.com/oracle/oci-service-operator/pkg/errorutil/errortest"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	"github.com/oracle/oci-service-operator/pkg/shared"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testWorkRequestID = "ocid1.jmsutilsworkrequest.oc1..wr"
	testWorkItemID    = "work-item-2"
)

type fakeWorkItemClient struct {
	responses []jmsutilssdk.ListWorkItemsResponse
	err       error
	requests  []jmsutilssdk.ListWorkItemsRequest
}

func (f *fakeWorkItemClient) ListWorkItems(
	_ context.Context,
	request jmsutilssdk.ListWorkItemsRequest,
) (jmsutilssdk.ListWorkItemsResponse, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return jmsutilssdk.ListWorkItemsResponse{}, f.err
	}
	index := len(f.requests) - 1
	if index >= len(f.responses) {
		return jmsutilssdk.ListWorkItemsResponse{}, errors.New("unexpected ListWorkItems call")
	}
	return f.responses[index], nil
}

func TestWorkItemRequiresWorkRequestIdentityBeforeOCI(t *testing.T) {
	fake := &fakeWorkItemClient{}
	client := newWorkItemServiceClientWithOCIClient(testLog(), fake)
	resource := newWorkItem(nil)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() err = nil, want missing work request error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = true, want false")
	}
	if len(fake.requests) != 0 {
		t.Fatalf("ListWorkItems calls = %d, want 0", len(fake.requests))
	}
	requireLastCondition(t, resource, shared.Failed)
	if !strings.Contains(resource.Status.OsokStatus.Message, workItemWorkRequestIDAnnotation) {
		t.Fatalf("status message = %q, want annotation guidance", resource.Status.OsokStatus.Message)
	}
}

func TestWorkItemBindsByAnnotationAcrossPagesAndProjectsStatus(t *testing.T) {
	timeLastUpdated := common.SDKTime{Time: metav1.NewTime(metav1.Now().Time).Time}
	fake := &fakeWorkItemClient{
		responses: []jmsutilssdk.ListWorkItemsResponse{
			{
				WorkItemCollection: jmsutilssdk.WorkItemCollection{
					Items: []jmsutilssdk.WorkItemSummary{
						makeWorkItemSummary("work-item-1", testWorkRequestID, jmsutilssdk.WorkItemStatusSucceeded, nil),
					},
				},
				OpcNextPage:  common.String("page-2"),
				OpcRequestId: common.String("opc-page-1"),
			},
			{
				WorkItemCollection: jmsutilssdk.WorkItemCollection{
					Items: []jmsutilssdk.WorkItemSummary{
						makeWorkItemSummary(testWorkItemID, testWorkRequestID, jmsutilssdk.WorkItemStatusSucceeded, &timeLastUpdated),
					},
				},
				OpcRequestId: common.String("opc-page-2"),
			},
		},
	}
	client := newWorkItemServiceClientWithOCIClient(testLog(), fake)
	resource := newWorkItem(map[string]string{
		workItemWorkRequestIDAnnotation: testWorkRequestID,
		workItemIDAnnotation:            testWorkItemID,
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() err = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = false, want true")
	}
	requirePaginatedWorkItemRequests(t, fake.requests)
	requireProjectedBoundWorkItem(t, resource)
}

func TestWorkItemNoopReconcileUsesRecordedIdentity(t *testing.T) {
	fake := &fakeWorkItemClient{
		responses: []jmsutilssdk.ListWorkItemsResponse{
			{
				WorkItemCollection: jmsutilssdk.WorkItemCollection{
					Items: []jmsutilssdk.WorkItemSummary{
						makeWorkItemSummary(testWorkItemID, testWorkRequestID, jmsutilssdk.WorkItemStatusSkipped, nil),
					},
				},
				OpcRequestId: common.String("opc-noop"),
			},
		},
	}
	client := newWorkItemServiceClientWithOCIClient(testLog(), fake)
	resource := newWorkItem(nil)
	resource.Status.Id = testWorkItemID
	resource.Status.WorkRequestId = testWorkRequestID
	resource.Status.OsokStatus.Ocid = shared.OCID(testWorkItemID)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() err = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = false, want true")
	}
	if len(fake.requests) != 1 {
		t.Fatalf("ListWorkItems calls = %d, want 1", len(fake.requests))
	}
	if got := resource.Status.Status; got != string(jmsutilssdk.WorkItemStatusSkipped) {
		t.Fatalf("status.sdkStatus = %q, want SKIPPED", got)
	}
	requireLastCondition(t, resource, shared.Active)
}

func TestWorkItemMultipleListItemsRequiresItemAnnotation(t *testing.T) {
	fake := &fakeWorkItemClient{
		responses: []jmsutilssdk.ListWorkItemsResponse{
			{
				WorkItemCollection: jmsutilssdk.WorkItemCollection{
					Items: []jmsutilssdk.WorkItemSummary{
						makeWorkItemSummary("work-item-1", testWorkRequestID, jmsutilssdk.WorkItemStatusSucceeded, nil),
						makeWorkItemSummary("work-item-2", testWorkRequestID, jmsutilssdk.WorkItemStatusSucceeded, nil),
					},
				},
			},
		},
	}
	client := newWorkItemServiceClientWithOCIClient(testLog(), fake)
	resource := newWorkItem(map[string]string{
		workItemWorkRequestIDAnnotation: testWorkRequestID,
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() err = nil, want ambiguous list error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = true, want false")
	}
	if !strings.Contains(err.Error(), workItemIDAnnotation) {
		t.Fatalf("error = %v, want work item annotation guidance", err)
	}
	requireLastCondition(t, resource, shared.Failed)
}

func TestWorkItemImmutableTrackedIdentityDriftIsRejectedBeforeOCI(t *testing.T) {
	fake := &fakeWorkItemClient{}
	client := newWorkItemServiceClientWithOCIClient(testLog(), fake)
	resource := newWorkItem(map[string]string{
		workItemWorkRequestIDAnnotation: "ocid1.jmsutilsworkrequest.oc1..replacement",
		workItemIDAnnotation:            testWorkItemID,
	})
	resource.Status.WorkRequestId = testWorkRequestID
	resource.Status.Id = testWorkItemID

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() err = nil, want immutable work request drift error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = true, want false")
	}
	if len(fake.requests) != 0 {
		t.Fatalf("ListWorkItems calls = %d, want 0", len(fake.requests))
	}
	if !strings.Contains(err.Error(), "workRequestId is immutable") {
		t.Fatalf("error = %v, want immutable workRequestId drift", err)
	}
	requireLastCondition(t, resource, shared.Failed)
}

func TestWorkItemPendingStatusTracksAsyncAndRequeues(t *testing.T) {
	fake := &fakeWorkItemClient{
		responses: []jmsutilssdk.ListWorkItemsResponse{
			{
				WorkItemCollection: jmsutilssdk.WorkItemCollection{
					Items: []jmsutilssdk.WorkItemSummary{
						makeWorkItemSummary(testWorkItemID, testWorkRequestID, jmsutilssdk.WorkItemStatusInProgress, nil),
					},
				},
			},
		},
	}
	client := newWorkItemServiceClientWithOCIClient(testLog(), fake)
	resource := newWorkItem(map[string]string{
		workItemWorkRequestIDAnnotation: testWorkRequestID,
		workItemIDAnnotation:            testWorkItemID,
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() err = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = false, want true for pending observation")
	}
	if !response.ShouldRequeue {
		t.Fatal("CreateOrUpdate() ShouldRequeue = false, want true")
	}
	current := resource.Status.OsokStatus.Async.Current
	if current == nil {
		t.Fatal("status.status.async.current = nil, want pending tracker")
	}
	if current.WorkRequestID != testWorkRequestID {
		t.Fatalf("async.current.workRequestId = %q, want %q", current.WorkRequestID, testWorkRequestID)
	}
	if current.RawStatus != string(jmsutilssdk.WorkItemStatusInProgress) {
		t.Fatalf("async.current.rawStatus = %q, want IN_PROGRESS", current.RawStatus)
	}
	if current.NormalizedClass != shared.OSOKAsyncClassPending {
		t.Fatalf("async.current.normalizedClass = %q, want pending", current.NormalizedClass)
	}
	requireLastCondition(t, resource, shared.Provisioning)
}

func TestWorkItemListErrorRecordsOpcRequestID(t *testing.T) {
	listErr := errortest.NewServiceError(500, errorutil.InternalServerError, "list failed")
	listErr.OpcRequestID = "opc-list-error"
	fake := &fakeWorkItemClient{err: listErr}
	client := newWorkItemServiceClientWithOCIClient(testLog(), fake)
	resource := newWorkItem(map[string]string{
		workItemWorkRequestIDAnnotation: testWorkRequestID,
		workItemIDAnnotation:            testWorkItemID,
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() err = nil, want list error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = true, want false")
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-list-error" {
		t.Fatalf("status.status.opcRequestId = %q, want opc-list-error", got)
	}
	requireLastCondition(t, resource, shared.Failed)
}

func TestWorkItemDeleteIsStatusOnlyAndDoesNotCallOCI(t *testing.T) {
	fake := &fakeWorkItemClient{}
	client := newWorkItemServiceClientWithOCIClient(testLog(), fake)
	resource := newWorkItem(map[string]string{
		workItemWorkRequestIDAnnotation: testWorkRequestID,
		workItemIDAnnotation:            testWorkItemID,
	})
	resource.Status.WorkRequestId = testWorkRequestID
	resource.Status.Id = testWorkItemID
	resource.Status.OsokStatus.Async.Current = &shared.OSOKAsyncOperation{
		Source:          shared.OSOKAsyncSourceLifecycle,
		Phase:           shared.OSOKAsyncPhaseCreate,
		WorkRequestID:   testWorkRequestID,
		RawStatus:       string(jmsutilssdk.WorkItemStatusInProgress),
		NormalizedClass: shared.OSOKAsyncClassPending,
	}

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() err = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() deleted = false, want true")
	}
	if len(fake.requests) != 0 {
		t.Fatalf("ListWorkItems calls = %d, want 0", len(fake.requests))
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("status.status.deletedAt = nil, want timestamp")
	}
	if resource.Status.OsokStatus.Async.Current != nil {
		t.Fatalf("status.status.async.current = %#v, want nil", resource.Status.OsokStatus.Async.Current)
	}
	requireLastCondition(t, resource, shared.Terminating)
}

func newWorkItem(annotations map[string]string) *jmsutilsv1beta1.WorkItem {
	return &jmsutilsv1beta1.WorkItem{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "workitem",
			Namespace:   "default",
			Annotations: annotations,
		},
	}
}

func makeWorkItemSummary(
	id string,
	workRequestID string,
	status jmsutilssdk.WorkItemStatusEnum,
	timeLastUpdated *common.SDKTime,
) jmsutilssdk.WorkItemSummary {
	retryCount := 2
	return jmsutilssdk.WorkItemSummary{
		Id:              common.String(id),
		WorkRequestId:   common.String(workRequestID),
		Status:          status,
		RetryCount:      &retryCount,
		TimeLastUpdated: timeLastUpdated,
		Details: jmsutilssdk.JavaMigrationWorkItemDetails{
			TargetJdkVersion:                    common.String("21"),
			InputApplicationsObjectStoragePaths: common.String("[\"/JMS/Utils/app.jar\"]"),
			AnalysisProjectName:                 common.String("analysis-project"),
			WorkItemType:                        jmsutilssdk.WorkItemTypeJavaMigration,
		},
	}
}

func requireLastCondition(
	t *testing.T,
	resource *jmsutilsv1beta1.WorkItem,
	want shared.OSOKConditionType,
) {
	t.Helper()
	conditions := resource.Status.OsokStatus.Conditions
	if len(conditions) == 0 {
		t.Fatalf("status.conditions = empty, want last condition %s", want)
	}
	last := conditions[len(conditions)-1]
	if last.Type != want {
		t.Fatalf("last condition = %s, want %s", last.Type, want)
	}
	if want == shared.Failed && last.Status != corev1.ConditionFalse {
		t.Fatalf("last condition status = %s, want False for Failed", last.Status)
	}
}

func requirePaginatedWorkItemRequests(t *testing.T, requests []jmsutilssdk.ListWorkItemsRequest) {
	t.Helper()
	if len(requests) != 2 {
		t.Fatalf("ListWorkItems calls = %d, want 2", len(requests))
	}
	if got := stringValue(requests[0].WorkRequestId); got != testWorkRequestID {
		t.Fatalf("first WorkRequestId = %q, want %q", got, testWorkRequestID)
	}
	if requests[0].Page != nil {
		t.Fatalf("first Page = %q, want nil", stringValue(requests[0].Page))
	}
	if got := stringValue(requests[1].Page); got != "page-2" {
		t.Fatalf("second Page = %q, want page-2", got)
	}
}

func requireProjectedBoundWorkItem(t *testing.T, resource *jmsutilsv1beta1.WorkItem) {
	t.Helper()
	if got := resource.Status.Id; got != testWorkItemID {
		t.Fatalf("status.id = %q, want %q", got, testWorkItemID)
	}
	if got := resource.Status.WorkRequestId; got != testWorkRequestID {
		t.Fatalf("status.workRequestId = %q, want %q", got, testWorkRequestID)
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != testWorkItemID {
		t.Fatalf("status.status.ocid = %q, want %q", got, testWorkItemID)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-page-2" {
		t.Fatalf("status.status.opcRequestId = %q, want opc-page-2", got)
	}
	if got := resource.Status.Details.Kind; got != string(jmsutilssdk.WorkItemDetailsKindJavaMigration) {
		t.Fatalf("status.details.kind = %q, want JAVA_MIGRATION", got)
	}
	if got := resource.Status.Details.TargetJdkVersion; got != "21" {
		t.Fatalf("status.details.targetJdkVersion = %q, want 21", got)
	}
	if resource.Status.TimeLastUpdated == "" {
		t.Fatal("status.timeLastUpdated = empty, want projected RFC3339 timestamp")
	}
	requireLastCondition(t, resource, shared.Active)
	if resource.Status.OsokStatus.Async.Current != nil {
		t.Fatalf("status.status.async.current = %#v, want nil for terminal success", resource.Status.OsokStatus.Async.Current)
	}
}

func testLog() loggerutil.OSOKLogger {
	return loggerutil.OSOKLogger{}
}

var _ servicemanager.OSOKServiceManager = (*WorkItemServiceManager)(nil)
