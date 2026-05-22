/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package performancetuninganalysis

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	jmsutilssdk "github.com/oracle/oci-go-sdk/v65/jmsutils"
	jmsutilsv1beta1 "github.com/oracle/oci-service-operator/api/jmsutils/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil"
	"github.com/oracle/oci-service-operator/pkg/errorutil/errortest"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testAnalysisID   = "ocid1.performancetuninganalysis.oc1..analysis"
	testCompartment  = "ocid1.compartment.oc1..compartment"
	testProject      = "analysis-project"
	testArtifactPath = "oci://bucket/path/app.jar"
	testWorkRequest  = "ocid1.workrequest.oc1..workrequest"
)

type fakePerformanceTuningAnalysisClient struct {
	requestCalls []jmsutilssdk.RequestPerformanceTuningAnalysisRequest
	getCalls     []jmsutilssdk.GetPerformanceTuningAnalysisRequest
	listCalls    []jmsutilssdk.ListPerformanceTuningAnalysisRequest
	deleteCalls  []jmsutilssdk.DeletePerformanceTuningAnalysisRequest
	workCalls    []jmsutilssdk.GetWorkRequestRequest

	requestResponse jmsutilssdk.RequestPerformanceTuningAnalysisResponse
	requestErr      error

	getResponses []jmsutilssdk.GetPerformanceTuningAnalysisResponse
	getErrs      []error

	listResponses []jmsutilssdk.ListPerformanceTuningAnalysisResponse
	listErrs      []error

	deleteResponse jmsutilssdk.DeletePerformanceTuningAnalysisResponse
	deleteErr      error

	workResponse jmsutilssdk.GetWorkRequestResponse
	workErr      error
}

func (f *fakePerformanceTuningAnalysisClient) RequestPerformanceTuningAnalysis(_ context.Context, request jmsutilssdk.RequestPerformanceTuningAnalysisRequest) (jmsutilssdk.RequestPerformanceTuningAnalysisResponse, error) {
	f.requestCalls = append(f.requestCalls, request)
	return f.requestResponse, f.requestErr
}

func (f *fakePerformanceTuningAnalysisClient) GetPerformanceTuningAnalysis(_ context.Context, request jmsutilssdk.GetPerformanceTuningAnalysisRequest) (jmsutilssdk.GetPerformanceTuningAnalysisResponse, error) {
	f.getCalls = append(f.getCalls, request)
	if len(f.getResponses) > 0 || len(f.getErrs) > 0 {
		response := jmsutilssdk.GetPerformanceTuningAnalysisResponse{}
		if len(f.getResponses) > 0 {
			response = f.getResponses[0]
			f.getResponses = f.getResponses[1:]
		}
		if len(f.getErrs) > 0 {
			err := f.getErrs[0]
			f.getErrs = f.getErrs[1:]
			return response, err
		}
		return response, nil
	}
	return jmsutilssdk.GetPerformanceTuningAnalysisResponse{}, nil
}

func (f *fakePerformanceTuningAnalysisClient) ListPerformanceTuningAnalysis(_ context.Context, request jmsutilssdk.ListPerformanceTuningAnalysisRequest) (jmsutilssdk.ListPerformanceTuningAnalysisResponse, error) {
	f.listCalls = append(f.listCalls, request)
	response := jmsutilssdk.ListPerformanceTuningAnalysisResponse{}
	if len(f.listResponses) > 0 {
		response = f.listResponses[0]
		f.listResponses = f.listResponses[1:]
	}
	if len(f.listErrs) > 0 {
		err := f.listErrs[0]
		f.listErrs = f.listErrs[1:]
		return response, err
	}
	return response, nil
}

func (f *fakePerformanceTuningAnalysisClient) DeletePerformanceTuningAnalysis(_ context.Context, request jmsutilssdk.DeletePerformanceTuningAnalysisRequest) (jmsutilssdk.DeletePerformanceTuningAnalysisResponse, error) {
	f.deleteCalls = append(f.deleteCalls, request)
	return f.deleteResponse, f.deleteErr
}

func (f *fakePerformanceTuningAnalysisClient) GetWorkRequest(_ context.Context, request jmsutilssdk.GetWorkRequestRequest) (jmsutilssdk.GetWorkRequestResponse, error) {
	f.workCalls = append(f.workCalls, request)
	return f.workResponse, f.workErr
}

func TestPerformanceTuningAnalysisCreateUsesAnnotationInputsAndTracksWorkRequest(t *testing.T) {
	client := &fakePerformanceTuningAnalysisClient{
		requestResponse: jmsutilssdk.RequestPerformanceTuningAnalysisResponse{
			OpcWorkRequestId: common.String(testWorkRequest),
			OpcRequestId:     common.String("opc-create"),
		},
	}
	resource := newPerformanceTuningAnalysisResource()

	response, err := newPerformanceTuningAnalysisServiceClientWithOCIClient(client, loggerutil.OSOKLogger{}).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || !response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v, want successful requeue", response)
	}
	if len(client.requestCalls) != 1 {
		t.Fatalf("RequestPerformanceTuningAnalysis calls = %d, want 1", len(client.requestCalls))
	}

	requireCreateRequest(t, client.requestCalls[0])
	requireCreateWorkRequestStatus(t, resource)
}

func TestPerformanceTuningAnalysisCreateBindsExistingAnalysisAfterPaginatedLookup(t *testing.T) {
	client := &fakePerformanceTuningAnalysisClient{
		listResponses: []jmsutilssdk.ListPerformanceTuningAnalysisResponse{
			{OpcNextPage: common.String("next")},
			{PerformanceTuningAnalysisCollection: jmsutilssdk.PerformanceTuningAnalysisCollection{
				Items: []jmsutilssdk.PerformanceTuningAnalysisSummary{
					newPerformanceTuningAnalysisSummary(testAnalysisID, testWorkRequest),
				},
			}},
		},
		getResponses: []jmsutilssdk.GetPerformanceTuningAnalysisResponse{
			{
				PerformanceTuningAnalysis: newPerformanceTuningAnalysis(testAnalysisID, testWorkRequest),
				OpcRequestId:              common.String("opc-get"),
			},
		},
	}
	resource := newPerformanceTuningAnalysisResource()

	response, err := newPerformanceTuningAnalysisServiceClientWithOCIClient(client, loggerutil.OSOKLogger{}).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v, want successful no requeue", response)
	}
	if len(client.requestCalls) != 0 {
		t.Fatalf("RequestPerformanceTuningAnalysis calls = %d, want 0", len(client.requestCalls))
	}
	if len(client.listCalls) != 2 {
		t.Fatalf("ListPerformanceTuningAnalysis calls = %d, want 2", len(client.listCalls))
	}
	if got := stringPtrValue(client.listCalls[1].Page); got != "next" {
		t.Fatalf("second list page = %q, want next", got)
	}
	if len(client.getCalls) != 1 || stringPtrValue(client.getCalls[0].PerformanceTuningAnalysisId) != testAnalysisID {
		t.Fatalf("get calls = %+v, want readback by bound id", client.getCalls)
	}
	requireProjectedAnalysis(t, resource, testAnalysisID, "opc-get")
}

func TestPerformanceTuningAnalysisCreateOrUpdateReadsRecordedIDWithoutDuplicateCreate(t *testing.T) {
	client := &fakePerformanceTuningAnalysisClient{
		getResponses: []jmsutilssdk.GetPerformanceTuningAnalysisResponse{{
			PerformanceTuningAnalysis: newPerformanceTuningAnalysis(testAnalysisID, testWorkRequest),
			OpcRequestId:              common.String("opc-observe"),
		}},
	}
	resource := newPerformanceTuningAnalysisResource()
	resource.Status.Id = testAnalysisID
	resource.Status.OsokStatus.Ocid = shared.OCID(testAnalysisID)

	response, err := newPerformanceTuningAnalysisServiceClientWithOCIClient(client, loggerutil.OSOKLogger{}).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v, want successful no requeue", response)
	}
	if len(client.requestCalls) != 0 || len(client.listCalls) != 0 {
		t.Fatalf("create/list calls = %d/%d, want 0/0", len(client.requestCalls), len(client.listCalls))
	}
	requireProjectedAnalysis(t, resource, testAnalysisID, "opc-observe")
}

func TestPerformanceTuningAnalysisRejectsCreateOnlyAnnotationDriftBeforeOCICall(t *testing.T) {
	client := &fakePerformanceTuningAnalysisClient{}
	resource := newPerformanceTuningAnalysisResource()
	resource.Status.CompartmentId = testCompartment
	resource.Annotations[PerformanceTuningAnalysisCompartmentIDAnnotation] = "ocid1.compartment.oc1..different"

	_, err := newPerformanceTuningAnalysisServiceClientWithOCIClient(client, loggerutil.OSOKLogger{}).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil || !strings.Contains(err.Error(), "create-only compartmentId annotation") {
		t.Fatalf("CreateOrUpdate() error = %v, want create-only drift rejection", err)
	}
	if len(client.requestCalls) != 0 || len(client.getCalls) != 0 || len(client.listCalls) != 0 {
		t.Fatalf("OCI calls = request:%d get:%d list:%d, want none", len(client.requestCalls), len(client.getCalls), len(client.listCalls))
	}
	if got := resource.Status.OsokStatus.Reason; got != string(shared.Failed) {
		t.Fatalf("status reason = %q, want Failed", got)
	}
}

func TestPerformanceTuningAnalysisResumeCreateWorkRequestProjectsCreatedAnalysis(t *testing.T) {
	client := &fakePerformanceTuningAnalysisClient{
		workResponse: jmsutilssdk.GetWorkRequestResponse{
			WorkRequest:  newPerformanceTuningAnalysisWorkRequest(jmsutilssdk.OperationStatusSucceeded, testAnalysisID),
			OpcRequestId: common.String("opc-work"),
		},
		getResponses: []jmsutilssdk.GetPerformanceTuningAnalysisResponse{{
			PerformanceTuningAnalysis: newPerformanceTuningAnalysis(testAnalysisID, testWorkRequest),
			OpcRequestId:              common.String("opc-created-get"),
		}},
	}
	resource := newPerformanceTuningAnalysisResource()
	resource.Status.WorkRequestId = testWorkRequest
	resource.Status.OsokStatus.Async.Current = &shared.OSOKAsyncOperation{
		Source:          shared.OSOKAsyncSourceWorkRequest,
		Phase:           shared.OSOKAsyncPhaseCreate,
		WorkRequestID:   testWorkRequest,
		NormalizedClass: shared.OSOKAsyncClassPending,
		UpdatedAt:       &metav1.Time{Time: time.Now()},
	}

	response, err := newPerformanceTuningAnalysisServiceClientWithOCIClient(client, loggerutil.OSOKLogger{}).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v, want successful completion", response)
	}
	if len(client.workCalls) != 1 || stringPtrValue(client.workCalls[0].WorkRequestId) != testWorkRequest {
		t.Fatalf("GetWorkRequest calls = %+v, want tracked work request", client.workCalls)
	}
	requireProjectedAnalysis(t, resource, testAnalysisID, "opc-created-get")
	if resource.Status.OsokStatus.Async.Current != nil {
		t.Fatalf("status async current = %+v, want cleared after readback", resource.Status.OsokStatus.Async.Current)
	}
}

func TestPerformanceTuningAnalysisResumeCreateWorkRequestIgnoresUnrelatedCreatedResource(t *testing.T) {
	client := &fakePerformanceTuningAnalysisClient{
		workResponse: jmsutilssdk.GetWorkRequestResponse{
			WorkRequest: jmsutilssdk.WorkRequest{
				OperationType: jmsutilssdk.OperationTypeRequestPerformanceTuningSaAnalysis,
				Status:        jmsutilssdk.OperationStatusSucceeded,
				Id:            common.String(testWorkRequest),
				Resources: []jmsutilssdk.WorkRequestResource{{
					EntityType: common.String("JavaMigrationAnalysis"),
					ActionType: jmsutilssdk.ActionTypeCreated,
					Identifier: common.String("ocid1.javamigrationanalysis.oc1..wrong"),
				}},
			},
			OpcRequestId: common.String("opc-work"),
		},
	}
	resource := newPerformanceTuningAnalysisResource()
	resource.Status.WorkRequestId = testWorkRequest
	resource.Status.OsokStatus.Async.Current = &shared.OSOKAsyncOperation{
		Source:          shared.OSOKAsyncSourceWorkRequest,
		Phase:           shared.OSOKAsyncPhaseCreate,
		WorkRequestID:   testWorkRequest,
		NormalizedClass: shared.OSOKAsyncClassPending,
		UpdatedAt:       &metav1.Time{Time: time.Now()},
	}

	response, err := newPerformanceTuningAnalysisServiceClientWithOCIClient(client, loggerutil.OSOKLogger{}).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || !response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v, want successful requeue while readback is absent", response)
	}
	if len(client.getCalls) != 0 {
		t.Fatalf("GetPerformanceTuningAnalysis calls = %d, want 0 for unrelated work-request resource", len(client.getCalls))
	}
	if len(client.listCalls) != 1 {
		t.Fatalf("ListPerformanceTuningAnalysis calls = %d, want lookup fallback", len(client.listCalls))
	}
	current := resource.Status.OsokStatus.Async.Current
	if current == nil || current.NormalizedClass != shared.OSOKAsyncClassPending || !strings.Contains(current.Message, "waiting for PerformanceTuningAnalysis") {
		t.Fatalf("status async current = %+v, want pending readback wait", current)
	}
}

func TestPerformanceTuningAnalysisDeleteWaitsForPendingCreateWorkRequest(t *testing.T) {
	client := &fakePerformanceTuningAnalysisClient{
		workResponse: jmsutilssdk.GetWorkRequestResponse{
			WorkRequest:  newPerformanceTuningAnalysisWorkRequest(jmsutilssdk.OperationStatusInProgress, ""),
			OpcRequestId: common.String("opc-work"),
		},
	}
	resource := newPerformanceTuningAnalysisResource()
	resource.Status.WorkRequestId = testWorkRequest
	resource.Status.OsokStatus.Async.Current = &shared.OSOKAsyncOperation{
		Source:          shared.OSOKAsyncSourceWorkRequest,
		Phase:           shared.OSOKAsyncPhaseCreate,
		WorkRequestID:   testWorkRequest,
		NormalizedClass: shared.OSOKAsyncClassPending,
		UpdatedAt:       &metav1.Time{Time: time.Now()},
	}

	deleted, err := newPerformanceTuningAnalysisServiceClientWithOCIClient(client, loggerutil.OSOKLogger{}).Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted {
		t.Fatal("Delete() deleted = true, want false while create work request is pending")
	}
	if len(client.deleteCalls) != 0 {
		t.Fatalf("delete calls = %d, want 0 before created resource identity is known", len(client.deleteCalls))
	}
	if got := resource.Status.OsokStatus.Reason; got != string(shared.Terminating) {
		t.Fatalf("status reason = %q, want Terminating", got)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-work" {
		t.Fatalf("status opcRequestId = %q, want opc-work", got)
	}
}

func TestPerformanceTuningAnalysisDeleteKeepsFinalizerUntilReadbackIsGone(t *testing.T) {
	client := &fakePerformanceTuningAnalysisClient{
		getResponses: []jmsutilssdk.GetPerformanceTuningAnalysisResponse{
			{PerformanceTuningAnalysis: newPerformanceTuningAnalysis(testAnalysisID, testWorkRequest)},
			{PerformanceTuningAnalysis: newPerformanceTuningAnalysis(testAnalysisID, testWorkRequest)},
		},
		deleteResponse: jmsutilssdk.DeletePerformanceTuningAnalysisResponse{OpcRequestId: common.String("opc-delete")},
	}
	resource := newPerformanceTuningAnalysisResource()
	resource.Status.Id = testAnalysisID
	resource.Status.OsokStatus.Ocid = shared.OCID(testAnalysisID)

	deleted, err := newPerformanceTuningAnalysisServiceClientWithOCIClient(client, loggerutil.OSOKLogger{}).Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted {
		t.Fatal("Delete() deleted = true, want false while readback still finds resource")
	}
	if len(client.deleteCalls) != 1 || stringPtrValue(client.deleteCalls[0].PerformanceTuningAnalysisId) != testAnalysisID {
		t.Fatalf("delete calls = %+v, want one delete by recorded id", client.deleteCalls)
	}
	if got := resource.Status.OsokStatus.Reason; got != string(shared.Terminating) {
		t.Fatalf("status reason = %q, want Terminating", got)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-delete" {
		t.Fatalf("status opcRequestId = %q, want opc-delete", got)
	}
	if resource.Status.OsokStatus.DeletedAt != nil {
		t.Fatalf("status deletedAt = %v, want nil until deletion is confirmed", resource.Status.OsokStatus.DeletedAt)
	}
}

func TestPerformanceTuningAnalysisDeleteConfirmsAfterUnambiguousNotFoundAndListAbsence(t *testing.T) {
	client := &fakePerformanceTuningAnalysisClient{
		getErrs: []error{errortest.NewServiceError(404, errorutil.NotFound, "gone")},
	}
	resource := newPerformanceTuningAnalysisResource()
	resource.Status.Id = testAnalysisID
	resource.Status.OsokStatus.Ocid = shared.OCID(testAnalysisID)
	resource.Status.CompartmentId = testCompartment
	resource.Status.AnalysisProjectName = testProject
	resource.Status.ArtifactObjectStoragePath = testArtifactPath

	deleted, err := newPerformanceTuningAnalysisServiceClientWithOCIClient(client, loggerutil.OSOKLogger{}).Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() deleted = false, want true after unambiguous readback/list absence")
	}
	if len(client.deleteCalls) != 0 {
		t.Fatalf("delete calls = %d, want 0 when already gone", len(client.deleteCalls))
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("status deletedAt = nil, want deletion confirmation timestamp")
	}
}

func TestPerformanceTuningAnalysisDeleteRejectsAuthShapedNotFound(t *testing.T) {
	client := &fakePerformanceTuningAnalysisClient{
		getErrs: []error{errortest.NewServiceError(404, errorutil.NotAuthorizedOrNotFound, "ambiguous")},
	}
	resource := newPerformanceTuningAnalysisResource()
	resource.Status.Id = testAnalysisID
	resource.Status.OsokStatus.Ocid = shared.OCID(testAnalysisID)

	deleted, err := newPerformanceTuningAnalysisServiceClientWithOCIClient(client, loggerutil.OSOKLogger{}).Delete(context.Background(), resource)
	if err == nil || !strings.Contains(err.Error(), "ambiguous 404 NotAuthorizedOrNotFound") {
		t.Fatalf("Delete() error = %v, want ambiguous not-found rejection", err)
	}
	if deleted {
		t.Fatal("Delete() deleted = true, want false for auth-shaped not-found")
	}
	if len(client.deleteCalls) != 0 {
		t.Fatalf("delete calls = %d, want 0 before deletion is confirmed safe", len(client.deleteCalls))
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-request-id" {
		t.Fatalf("status opcRequestId = %q, want error request id", got)
	}
}

func TestPerformanceTuningAnalysisCreateRecordsOCIErrorRequestID(t *testing.T) {
	serviceErr := errortest.NewServiceError(500, errorutil.InternalServerError, "service unavailable")
	client := &fakePerformanceTuningAnalysisClient{requestErr: serviceErr}
	resource := newPerformanceTuningAnalysisResource()

	_, err := newPerformanceTuningAnalysisServiceClientWithOCIClient(client, loggerutil.OSOKLogger{}).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if !errors.Is(err, serviceErr) {
		t.Fatalf("CreateOrUpdate() error = %v, want service error", err)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-request-id" {
		t.Fatalf("status opcRequestId = %q, want service error request id", got)
	}
	if got := resource.Status.OsokStatus.Reason; got != string(shared.Failed) {
		t.Fatalf("status reason = %q, want Failed", got)
	}
}

func TestPerformanceTuningAnalysisRuntimeHooksInstallManualWrapper(t *testing.T) {
	hooks := newPerformanceTuningAnalysisDefaultRuntimeHooks(jmsutilssdk.JmsUtilsClient{})
	applyPerformanceTuningAnalysisRuntimeHooks(&PerformanceTuningAnalysisServiceManager{}, &hooks, &fakePerformanceTuningAnalysisClient{}, nil)

	if hooks.Semantics == nil {
		t.Fatal("Semantics = nil, want resource-local runtime semantics")
	}
	if len(hooks.WrapGeneratedClient) != 1 {
		t.Fatalf("WrapGeneratedClient count = %d, want 1", len(hooks.WrapGeneratedClient))
	}
	wrapped := hooks.WrapGeneratedClient[0](nil)
	if _, ok := wrapped.(*performanceTuningAnalysisRuntimeClient); !ok {
		t.Fatalf("wrapped client type = %T, want *performanceTuningAnalysisRuntimeClient", wrapped)
	}
	if got, want := hooks.Semantics.Mutation.ForceNew, []string{"compartmentId", "analysisProjectName", "artifactObjectStoragePath"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("forceNew = %v, want %v", got, want)
	}
}

func newPerformanceTuningAnalysisResource() *jmsutilsv1beta1.PerformanceTuningAnalysis {
	return &jmsutilsv1beta1.PerformanceTuningAnalysis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pta",
			Namespace: "default",
			UID:       types.UID("resource-uid"),
			Annotations: map[string]string{
				PerformanceTuningAnalysisCompartmentIDAnnotation:             testCompartment,
				PerformanceTuningAnalysisAnalysisProjectNameAnnotation:       testProject,
				PerformanceTuningAnalysisArtifactObjectStoragePathAnnotation: testArtifactPath,
			},
		},
	}
}

func newPerformanceTuningAnalysis(id string, workRequestID string) jmsutilssdk.PerformanceTuningAnalysis {
	now := common.SDKTime{Time: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)}
	warningCount := 7
	return jmsutilssdk.PerformanceTuningAnalysis{
		Id:                        common.String(id),
		WorkRequestId:             common.String(workRequestID),
		CompartmentId:             common.String(testCompartment),
		AnalysisProjectName:       common.String(testProject),
		WarningCount:              common.Int(warningCount),
		Result:                    jmsutilssdk.PerformanceTuningAnalysisResultActionRecommended,
		ResultObjectStoragePath:   common.String("oci://bucket/path/result.json"),
		ArtifactObjectStoragePath: common.String(testArtifactPath),
		TimeCreated:               &now,
		TimeStarted:               &now,
		TimeFinished:              &now,
		CreatedBy: &jmsutilssdk.Principal{
			Id:          common.String("ocid1.user.oc1..user"),
			DisplayName: common.String("user"),
		},
	}
}

func newPerformanceTuningAnalysisSummary(id string, workRequestID string) jmsutilssdk.PerformanceTuningAnalysisSummary {
	now := common.SDKTime{Time: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)}
	warningCount := 7
	return jmsutilssdk.PerformanceTuningAnalysisSummary{
		Id:                        common.String(id),
		WorkRequestId:             common.String(workRequestID),
		CompartmentId:             common.String(testCompartment),
		AnalysisProjectName:       common.String(testProject),
		WarningCount:              common.Int(warningCount),
		Result:                    jmsutilssdk.PerformanceTuningAnalysisResultActionRecommended,
		ResultObjectStoragePath:   common.String("oci://bucket/path/result.json"),
		ArtifactObjectStoragePath: common.String(testArtifactPath),
		TimeCreated:               &now,
		TimeStarted:               &now,
		TimeFinished:              &now,
		CreatedBy: &jmsutilssdk.Principal{
			Id:          common.String("ocid1.user.oc1..user"),
			DisplayName: common.String("user"),
		},
	}
}

func newPerformanceTuningAnalysisWorkRequest(status jmsutilssdk.OperationStatusEnum, resourceID string) jmsutilssdk.WorkRequest {
	resources := []jmsutilssdk.WorkRequestResource{}
	if resourceID != "" {
		resources = append(resources, jmsutilssdk.WorkRequestResource{
			EntityType: common.String("PerformanceTuningAnalysis"),
			ActionType: jmsutilssdk.ActionTypeCreated,
			Identifier: common.String(resourceID),
		})
	}
	return jmsutilssdk.WorkRequest{
		OperationType:   jmsutilssdk.OperationTypeRequestPerformanceTuningSaAnalysis,
		Status:          status,
		Id:              common.String(testWorkRequest),
		CompartmentId:   common.String(testCompartment),
		Resources:       resources,
		PercentComplete: common.Float32(100),
	}
}

func requireProjectedAnalysis(t *testing.T, resource *jmsutilsv1beta1.PerformanceTuningAnalysis, id string, opcRequestID string) {
	t.Helper()

	if got := resource.Status.Id; got != id {
		t.Fatalf("status id = %q, want %q", got, id)
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != id {
		t.Fatalf("status.status.ocid = %q, want %q", got, id)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != opcRequestID {
		t.Fatalf("status.status.opcRequestId = %q, want %q", got, opcRequestID)
	}
	if got := resource.Status.OsokStatus.Reason; got != string(shared.Active) {
		t.Fatalf("status reason = %q, want Active", got)
	}
	if got := resource.Status.WarningCount; got != 7 {
		t.Fatalf("warningCount = %d, want 7", got)
	}
	if got := resource.Status.CreatedBy.DisplayName; got != "user" {
		t.Fatalf("createdBy.displayName = %q, want user", got)
	}
}

func requireCreateRequest(t *testing.T, request jmsutilssdk.RequestPerformanceTuningAnalysisRequest) {
	t.Helper()

	if got := stringPtrValue(request.CompartmentId); got != testCompartment {
		t.Fatalf("request compartmentId = %q, want %q", got, testCompartment)
	}
	if len(request.Targets) != 1 {
		t.Fatalf("request targets = %d, want 1", len(request.Targets))
	}
	if got := stringPtrValue(request.Targets[0].AnalysisProjectName); got != testProject {
		t.Fatalf("request target analysisProjectName = %q, want %q", got, testProject)
	}
	if got := stringPtrValue(request.Targets[0].ArtifactObjectStoragePath); got != testArtifactPath {
		t.Fatalf("request target artifactObjectStoragePath = %q, want %q", got, testArtifactPath)
	}
	if got := stringPtrValue(request.OpcRetryToken); !strings.HasPrefix(got, "osok-pta-") {
		t.Fatalf("opc retry token = %q, want OSOK deterministic token", got)
	}
}

func requireCreateWorkRequestStatus(t *testing.T, resource *jmsutilsv1beta1.PerformanceTuningAnalysis) {
	t.Helper()

	if got := resource.Status.WorkRequestId; got != testWorkRequest {
		t.Fatalf("status workRequestId = %q, want %q", got, testWorkRequest)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-create" {
		t.Fatalf("status opcRequestId = %q, want opc-create", got)
	}
	if current := resource.Status.OsokStatus.Async.Current; current == nil || current.WorkRequestID != testWorkRequest || current.Phase != shared.OSOKAsyncPhaseCreate {
		t.Fatalf("status async current = %+v, want create work request", current)
	}
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
