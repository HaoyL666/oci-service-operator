/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package javamigrationanalysis

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	jmsutilssdk "github.com/oracle/oci-go-sdk/v65/jmsutils"
	jmsutilsv1beta1 "github.com/oracle/oci-service-operator/api/jmsutils/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil"
	"github.com/oracle/oci-service-operator/pkg/errorutil/errortest"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testJavaMigrationAnalysisID = "ocid1.javamigrationanalysis.oc1..example"
	testCompartmentID           = "ocid1.compartment.oc1..example"
	testProjectName             = "migration-project"
	testWorkRequestID           = "ocid1.jmsutilsworkrequest.oc1..example"
)

func TestJavaMigrationAnalysisCreateOrUpdateRequestsCreateWithRetryToken(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.list = func(_ context.Context, request jmsutilssdk.ListJavaMigrationAnalysisRequest) (jmsutilssdk.ListJavaMigrationAnalysisResponse, error) {
		assertJavaMigrationAnalysisListRequest(t, request)
		return jmsutilssdk.ListJavaMigrationAnalysisResponse{}, nil
	}
	fake.request = func(_ context.Context, request jmsutilssdk.RequestJavaMigrationAnalysisRequest) (jmsutilssdk.RequestJavaMigrationAnalysisResponse, error) {
		assertJavaMigrationAnalysisCreateRequest(t, request)
		return jmsutilssdk.RequestJavaMigrationAnalysisResponse{
			OpcWorkRequestId: common.String(testWorkRequestID),
			OpcRequestId:     common.String("create-request"),
		}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := makeJavaMigrationAnalysisResourceWithSpec()
	resource.Name = "sample"
	resource.Namespace = "default"
	resource.UID = "11111111-2222-3333-4444-555555555555"

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	assertJavaMigrationAnalysisPendingCreate(t, response, resource, fake)
	requireLatestCondition(t, resource, shared.Provisioning)
}

func TestJavaMigrationAnalysisCreateOrUpdateRejectsMissingSpecBeforeOCI(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("CreateOrUpdate() error = %v, want required spec error", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() IsSuccessful = true, want false")
	}
	if got := fake.totalCalls(); got != 0 {
		t.Fatalf("OCI calls = %d, want 0 before spec rejection", got)
	}
	requireLatestCondition(t, resource, shared.Failed)
}

func TestJavaMigrationAnalysisCreateOrUpdateObservesRecordedID(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.get = func(_ context.Context, request jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		if got := stringValue(request.JavaMigrationAnalysisId); got != testJavaMigrationAnalysisID {
			t.Fatalf("GetJavaMigrationAnalysis id = %q, want %q", got, testJavaMigrationAnalysisID)
		}
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{
			JavaMigrationAnalysis: makeSDKJavaMigrationAnalysis(testJavaMigrationAnalysisID),
			OpcRequestId:          common.String("get-request"),
		}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}
	resource.Status.OsokStatus.Ocid = shared.OCID(testJavaMigrationAnalysisID)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v, want successful without requeue", response)
	}
	if got := resource.Status.Id; got != testJavaMigrationAnalysisID {
		t.Fatalf("status.id = %q, want %q", got, testJavaMigrationAnalysisID)
	}
	if got := resource.Status.AnalysisProjectName; got != testProjectName {
		t.Fatalf("status.analysisProjectName = %q, want %q", got, testProjectName)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "get-request" {
		t.Fatalf("status.status.opcRequestId = %q, want get-request", got)
	}
	requireLatestCondition(t, resource, shared.Active)
}

func TestJavaMigrationAnalysisCreateOrUpdateObservesAnnotationID(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.get = func(_ context.Context, request jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		if got := stringValue(request.JavaMigrationAnalysisId); got != testJavaMigrationAnalysisID {
			t.Fatalf("GetJavaMigrationAnalysis id = %q, want %q", got, testJavaMigrationAnalysisID)
		}
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{
			JavaMigrationAnalysis: makeSDKJavaMigrationAnalysis(testJavaMigrationAnalysisID),
		}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				javaMigrationAnalysisIDAnnotation: testJavaMigrationAnalysisID,
			},
		},
	}

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() IsSuccessful = false, want true")
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != testJavaMigrationAnalysisID {
		t.Fatalf("status.status.ocid = %q, want %q", got, testJavaMigrationAnalysisID)
	}
	requireLatestCondition(t, resource, shared.Active)
}

func TestJavaMigrationAnalysisCreateOrUpdateRejectsAnnotationIdentityDrift(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				javaMigrationAnalysisIDAnnotation: "ocid1.javamigrationanalysis.oc1..other",
			},
		},
	}
	resource.Status.OsokStatus.Ocid = shared.OCID(testJavaMigrationAnalysisID)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil || !strings.Contains(err.Error(), "create-only") {
		t.Fatalf("CreateOrUpdate() error = %v, want create-only identity drift", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() IsSuccessful = true, want false")
	}
	if got := fake.totalCalls(); got != 0 {
		t.Fatalf("OCI calls = %d, want 0 before identity drift rejection", got)
	}
	requireLatestCondition(t, resource, shared.Failed)
}

func TestJavaMigrationAnalysisCreateOrUpdateRecordsOCIErrorRequestID(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.get = func(context.Context, jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{}, errortest.NewServiceError(500, "InternalError", "read failed")
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}
	resource.Status.OsokStatus.Ocid = shared.OCID(testJavaMigrationAnalysisID)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatalf("CreateOrUpdate() error = nil, want OCI error")
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() IsSuccessful = true, want false")
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-request-id" {
		t.Fatalf("status.status.opcRequestId = %q, want opc-request-id", got)
	}
	requireLatestCondition(t, resource, shared.Failed)
}

func TestJavaMigrationAnalysisCreateOrUpdateRejectsCreateOnlyDrift(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.get = func(context.Context, jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{
			JavaMigrationAnalysis: makeSDKJavaMigrationAnalysis(testJavaMigrationAnalysisID),
		}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := makeJavaMigrationAnalysisResourceWithSpec()
	resource.Spec.TargetJdkVersion = "21"
	resource.Status.OsokStatus.Ocid = shared.OCID(testJavaMigrationAnalysisID)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil || !strings.Contains(err.Error(), "create-only drift") {
		t.Fatalf("CreateOrUpdate() error = %v, want create-only drift", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() IsSuccessful = true, want false")
	}
	requireLatestCondition(t, resource, shared.Failed)
}

func TestJavaMigrationAnalysisCreateOrUpdateBindsByPaginatedSpecList(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.list = func(_ context.Context, request jmsutilssdk.ListJavaMigrationAnalysisRequest) (jmsutilssdk.ListJavaMigrationAnalysisResponse, error) {
		if got := stringValue(request.CompartmentId); got != testCompartmentID {
			t.Fatalf("ListJavaMigrationAnalysis compartmentId = %q, want %q", got, testCompartmentID)
		}
		if got := stringValue(request.AnalysisProjectName); got != testProjectName {
			t.Fatalf("ListJavaMigrationAnalysis analysisProjectName = %q, want %q", got, testProjectName)
		}
		switch stringValue(request.Page) {
		case "":
			return jmsutilssdk.ListJavaMigrationAnalysisResponse{
				JavaMigrationAnalysisCollection: jmsutilssdk.JavaMigrationAnalysisCollection{
					Items: []jmsutilssdk.JavaMigrationAnalysisSummary{
						makeSDKJavaMigrationAnalysisSummary("ocid1.javamigrationanalysis.oc1..other", testCompartmentID, "other-project"),
					},
				},
				OpcNextPage: common.String("page-2"),
			}, nil
		case "page-2":
			return jmsutilssdk.ListJavaMigrationAnalysisResponse{
				JavaMigrationAnalysisCollection: jmsutilssdk.JavaMigrationAnalysisCollection{
					Items: []jmsutilssdk.JavaMigrationAnalysisSummary{
						makeSDKJavaMigrationAnalysisSummary(testJavaMigrationAnalysisID, testCompartmentID, testProjectName),
					},
				},
			}, nil
		default:
			t.Fatalf("unexpected list page %q", stringValue(request.Page))
			return jmsutilssdk.ListJavaMigrationAnalysisResponse{}, nil
		}
	}
	fake.get = func(_ context.Context, request jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{
			JavaMigrationAnalysis: makeSDKJavaMigrationAnalysis(stringValue(request.JavaMigrationAnalysisId)),
		}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := makeJavaMigrationAnalysisResourceWithSpec()

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() IsSuccessful = false, want true")
	}
	if got := len(fake.listRequests); got != 2 {
		t.Fatalf("list calls = %d, want 2", got)
	}
	if got := len(fake.requestRequests); got != 0 {
		t.Fatalf("request calls = %d, want 0 after bind", got)
	}
	if got := resource.Status.Id; got != testJavaMigrationAnalysisID {
		t.Fatalf("status.id = %q, want %q", got, testJavaMigrationAnalysisID)
	}
}

func TestJavaMigrationAnalysisCreateOrUpdateBindsByPaginatedStatusList(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.list = func(_ context.Context, request jmsutilssdk.ListJavaMigrationAnalysisRequest) (jmsutilssdk.ListJavaMigrationAnalysisResponse, error) {
		if got := stringValue(request.CompartmentId); got != testCompartmentID {
			t.Fatalf("ListJavaMigrationAnalysis compartmentId = %q, want %q", got, testCompartmentID)
		}
		if got := stringValue(request.AnalysisProjectName); got != testProjectName {
			t.Fatalf("ListJavaMigrationAnalysis analysisProjectName = %q, want %q", got, testProjectName)
		}
		switch stringValue(request.Page) {
		case "":
			return jmsutilssdk.ListJavaMigrationAnalysisResponse{
				JavaMigrationAnalysisCollection: jmsutilssdk.JavaMigrationAnalysisCollection{
					Items: []jmsutilssdk.JavaMigrationAnalysisSummary{
						makeSDKJavaMigrationAnalysisSummary("ocid1.javamigrationanalysis.oc1..other", testCompartmentID, "other-project"),
					},
				},
				OpcNextPage: common.String("page-2"),
			}, nil
		case "page-2":
			return jmsutilssdk.ListJavaMigrationAnalysisResponse{
				JavaMigrationAnalysisCollection: jmsutilssdk.JavaMigrationAnalysisCollection{
					Items: []jmsutilssdk.JavaMigrationAnalysisSummary{
						makeSDKJavaMigrationAnalysisSummary(testJavaMigrationAnalysisID, testCompartmentID, testProjectName),
					},
				},
			}, nil
		default:
			t.Fatalf("unexpected list page %q", stringValue(request.Page))
			return jmsutilssdk.ListJavaMigrationAnalysisResponse{}, nil
		}
	}
	fake.get = func(_ context.Context, request jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{
			JavaMigrationAnalysis: makeSDKJavaMigrationAnalysis(stringValue(request.JavaMigrationAnalysisId)),
		}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}
	resource.Status.CompartmentId = testCompartmentID
	resource.Status.AnalysisProjectName = testProjectName

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() IsSuccessful = false, want true")
	}
	if got := len(fake.listRequests); got != 2 {
		t.Fatalf("list calls = %d, want 2", got)
	}
	if got := stringValue(fake.listRequests[1].Page); got != "page-2" {
		t.Fatalf("second list page = %q, want page-2", got)
	}
	if got := resource.Status.Id; got != testJavaMigrationAnalysisID {
		t.Fatalf("status.id = %q, want %q", got, testJavaMigrationAnalysisID)
	}
}

func TestJavaMigrationAnalysisCreateOrUpdateRejectsAmbiguousStatusBind(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.list = func(context.Context, jmsutilssdk.ListJavaMigrationAnalysisRequest) (jmsutilssdk.ListJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.ListJavaMigrationAnalysisResponse{
			JavaMigrationAnalysisCollection: jmsutilssdk.JavaMigrationAnalysisCollection{
				Items: []jmsutilssdk.JavaMigrationAnalysisSummary{
					makeSDKJavaMigrationAnalysisSummary("ocid1.javamigrationanalysis.oc1..first", testCompartmentID, testProjectName),
					makeSDKJavaMigrationAnalysisSummary("ocid1.javamigrationanalysis.oc1..second", testCompartmentID, testProjectName),
				},
			},
		}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}
	resource.Status.CompartmentId = testCompartmentID
	resource.Status.AnalysisProjectName = testProjectName

	_, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil || !strings.Contains(err.Error(), "multiple JavaMigrationAnalysis resources matched") {
		t.Fatalf("CreateOrUpdate() error = %v, want ambiguous bind error", err)
	}
	if got := len(fake.getRequests); got != 0 {
		t.Fatalf("get calls = %d, want 0 after ambiguous bind", got)
	}
	requireLatestCondition(t, resource, shared.Failed)
}

func TestJavaMigrationAnalysisCreateOrUpdateTracksPendingWorkRequest(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.getWorkRequest = func(_ context.Context, request jmsutilssdk.GetWorkRequestRequest) (jmsutilssdk.GetWorkRequestResponse, error) {
		if got := stringValue(request.WorkRequestId); got != testWorkRequestID {
			t.Fatalf("GetWorkRequest id = %q, want %q", got, testWorkRequestID)
		}
		percent := float32(35)
		return jmsutilssdk.GetWorkRequestResponse{
			WorkRequest: jmsutilssdk.WorkRequest{
				Id:              common.String(testWorkRequestID),
				Status:          jmsutilssdk.OperationStatusInProgress,
				OperationType:   jmsutilssdk.OperationTypeRequestJavaMigrationSaAnalysis,
				PercentComplete: &percent,
			},
			OpcRequestId: common.String("work-request-get"),
		}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}
	resource.Status.WorkRequestId = testWorkRequestID

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || !response.ShouldRequeue || response.RequeueDuration != javaMigrationAnalysisPollInterval {
		t.Fatalf("CreateOrUpdate() response = %+v, want pending requeue", response)
	}
	current := resource.Status.OsokStatus.Async.Current
	if current == nil || current.WorkRequestID != testWorkRequestID || current.NormalizedClass != shared.OSOKAsyncClassPending {
		t.Fatalf("async.current = %+v, want pending work request %s", current, testWorkRequestID)
	}
	requireLatestCondition(t, resource, shared.Provisioning)
}

func TestJavaMigrationAnalysisCreateOrUpdateCompletesWorkRequestAndReadsResource(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.getWorkRequest = func(context.Context, jmsutilssdk.GetWorkRequestRequest) (jmsutilssdk.GetWorkRequestResponse, error) {
		return jmsutilssdk.GetWorkRequestResponse{
			WorkRequest: jmsutilssdk.WorkRequest{
				Id:            common.String(testWorkRequestID),
				Status:        jmsutilssdk.OperationStatusSucceeded,
				OperationType: jmsutilssdk.OperationTypeRequestJavaMigrationSaAnalysis,
				Resources: []jmsutilssdk.WorkRequestResource{
					{
						EntityType: common.String("JavaMigrationAnalysis"),
						ActionType: jmsutilssdk.ActionTypeCreated,
						Identifier: common.String(testJavaMigrationAnalysisID),
					},
				},
			},
		}, nil
	}
	fake.get = func(_ context.Context, request jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{
			JavaMigrationAnalysis: makeSDKJavaMigrationAnalysis(stringValue(request.JavaMigrationAnalysisId)),
		}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}
	resource.Status.WorkRequestId = testWorkRequestID

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v, want completed readback", response)
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != testJavaMigrationAnalysisID {
		t.Fatalf("status.status.ocid = %q, want %q", got, testJavaMigrationAnalysisID)
	}
	requireLatestCondition(t, resource, shared.Active)
}

func TestJavaMigrationAnalysisCreateOrUpdateCompletesWorkRequestAndBindsBySpecWhenWorkRequestHasNoResource(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.getWorkRequest = func(context.Context, jmsutilssdk.GetWorkRequestRequest) (jmsutilssdk.GetWorkRequestResponse, error) {
		return jmsutilssdk.GetWorkRequestResponse{
			WorkRequest: jmsutilssdk.WorkRequest{
				Id:            common.String(testWorkRequestID),
				Status:        jmsutilssdk.OperationStatusSucceeded,
				OperationType: jmsutilssdk.OperationTypeRequestJavaMigrationSaAnalysis,
			},
		}, nil
	}
	fake.list = func(context.Context, jmsutilssdk.ListJavaMigrationAnalysisRequest) (jmsutilssdk.ListJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.ListJavaMigrationAnalysisResponse{
			JavaMigrationAnalysisCollection: jmsutilssdk.JavaMigrationAnalysisCollection{
				Items: []jmsutilssdk.JavaMigrationAnalysisSummary{
					makeSDKJavaMigrationAnalysisSummary(testJavaMigrationAnalysisID, testCompartmentID, testProjectName),
				},
			},
		}, nil
	}
	fake.get = func(_ context.Context, request jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{
			JavaMigrationAnalysis: makeSDKJavaMigrationAnalysis(stringValue(request.JavaMigrationAnalysisId)),
		}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := makeJavaMigrationAnalysisResourceWithSpec()
	resource.Status.WorkRequestId = testWorkRequestID

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v, want completed readback", response)
	}
	if got := len(fake.listRequests); got != 1 {
		t.Fatalf("list calls = %d, want 1 fallback readback lookup", got)
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != testJavaMigrationAnalysisID {
		t.Fatalf("status.status.ocid = %q, want %q", got, testJavaMigrationAnalysisID)
	}
	requireLatestCondition(t, resource, shared.Active)
}

func TestJavaMigrationAnalysisDeleteConfirmsUnambiguousNotFound(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.get = func(_ context.Context, request jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		if len(fake.getRequests) == 1 {
			return jmsutilssdk.GetJavaMigrationAnalysisResponse{
				JavaMigrationAnalysis: makeSDKJavaMigrationAnalysis(stringValue(request.JavaMigrationAnalysisId)),
				Etag:                  common.String("etag-value"),
			}, nil
		}
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{}, errortest.NewServiceError(404, errorutil.NotFound, "gone")
	}
	fake.delete = func(_ context.Context, request jmsutilssdk.DeleteJavaMigrationAnalysisRequest) (jmsutilssdk.DeleteJavaMigrationAnalysisResponse, error) {
		if got := stringValue(request.IfMatch); got != "etag-value" {
			t.Fatalf("DeleteJavaMigrationAnalysis ifMatch = %q, want etag-value", got)
		}
		return jmsutilssdk.DeleteJavaMigrationAnalysisResponse{OpcRequestId: common.String("delete-request")}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}
	resource.Status.OsokStatus.Ocid = shared.OCID(testJavaMigrationAnalysisID)

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatalf("Delete() deleted = false, want true")
	}
	if got := len(fake.deleteRequests); got != 1 {
		t.Fatalf("delete calls = %d, want 1", got)
	}
	requireLatestCondition(t, resource, shared.Terminating)
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatalf("status.status.deletedAt is nil, want deletion timestamp")
	}
}

func TestJavaMigrationAnalysisDeleteRejectsAuthShapedConfirmRead(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.get = func(_ context.Context, request jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		if len(fake.getRequests) == 1 {
			return jmsutilssdk.GetJavaMigrationAnalysisResponse{
				JavaMigrationAnalysis: makeSDKJavaMigrationAnalysis(stringValue(request.JavaMigrationAnalysisId)),
			}, nil
		}
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{}, errortest.NewServiceError(404, errorutil.NotAuthorizedOrNotFound, "ambiguous")
	}
	fake.delete = func(context.Context, jmsutilssdk.DeleteJavaMigrationAnalysisRequest) (jmsutilssdk.DeleteJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.DeleteJavaMigrationAnalysisResponse{}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}
	resource.Status.OsokStatus.Ocid = shared.OCID(testJavaMigrationAnalysisID)

	deleted, err := client.Delete(context.Background(), resource)
	if err == nil || !strings.Contains(err.Error(), "ambiguous 404 NotAuthorizedOrNotFound") {
		t.Fatalf("Delete() error = %v, want ambiguous auth-shaped not-found error", err)
	}
	if deleted {
		t.Fatalf("Delete() deleted = true, want false")
	}
	requireLatestCondition(t, resource, shared.Terminating)
	if current := resource.Status.OsokStatus.Async.Current; current == nil || current.Phase != shared.OSOKAsyncPhaseDelete {
		t.Fatalf("async.current = %+v, want pending delete", current)
	}
}

func TestJavaMigrationAnalysisDeleteKeepsFinalizerUntilReadbackMissing(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.get = func(_ context.Context, request jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{
			JavaMigrationAnalysis: makeSDKJavaMigrationAnalysis(stringValue(request.JavaMigrationAnalysisId)),
		}, nil
	}
	fake.delete = func(context.Context, jmsutilssdk.DeleteJavaMigrationAnalysisRequest) (jmsutilssdk.DeleteJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.DeleteJavaMigrationAnalysisResponse{}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}
	resource.Status.OsokStatus.Ocid = shared.OCID(testJavaMigrationAnalysisID)

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted {
		t.Fatalf("Delete() deleted = true, want false while OCI readback still returns resource")
	}
	requireLatestCondition(t, resource, shared.Terminating)
	if current := resource.Status.OsokStatus.Async.Current; current == nil || current.Phase != shared.OSOKAsyncPhaseDelete {
		t.Fatalf("async.current = %+v, want pending delete", current)
	}
}

func TestJavaMigrationAnalysisDeletePendingOnlyConfirmsReadback(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	fake.get = func(_ context.Context, request jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{
			JavaMigrationAnalysis: makeSDKJavaMigrationAnalysis(stringValue(request.JavaMigrationAnalysisId)),
		}, nil
	}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}
	resource.Status.OsokStatus.Ocid = shared.OCID(testJavaMigrationAnalysisID)
	resource.Status.OsokStatus.Async.Current = &shared.OSOKAsyncOperation{
		Source:          shared.OSOKAsyncSourceLifecycle,
		Phase:           shared.OSOKAsyncPhaseDelete,
		NormalizedClass: shared.OSOKAsyncClassPending,
		UpdatedAt:       &metav1.Time{Time: time.Now()},
	}

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted {
		t.Fatalf("Delete() deleted = true, want false while OCI readback still returns resource")
	}
	if got := len(fake.deleteRequests); got != 0 {
		t.Fatalf("delete calls = %d, want 0 while delete confirmation is pending", got)
	}
	if got := len(fake.getRequests); got != 1 {
		t.Fatalf("get calls = %d, want 1 confirmation read", got)
	}
	requireLatestCondition(t, resource, shared.Terminating)
}

func TestJavaMigrationAnalysisDeleteWithoutIdentityRemovesFinalizer(t *testing.T) {
	fake := &fakeJavaMigrationAnalysisOCIClient{}
	client := newJavaMigrationAnalysisRuntimeClientForTest(fake)
	resource := &jmsutilsv1beta1.JavaMigrationAnalysis{}

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatalf("Delete() deleted = false, want true without recorded identity")
	}
	if got := fake.totalCalls(); got != 0 {
		t.Fatalf("OCI calls = %d, want 0 without recorded identity", got)
	}
	requireLatestCondition(t, resource, shared.Terminating)
}

type fakeJavaMigrationAnalysisOCIClient struct {
	request         func(context.Context, jmsutilssdk.RequestJavaMigrationAnalysisRequest) (jmsutilssdk.RequestJavaMigrationAnalysisResponse, error)
	get             func(context.Context, jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error)
	list            func(context.Context, jmsutilssdk.ListJavaMigrationAnalysisRequest) (jmsutilssdk.ListJavaMigrationAnalysisResponse, error)
	delete          func(context.Context, jmsutilssdk.DeleteJavaMigrationAnalysisRequest) (jmsutilssdk.DeleteJavaMigrationAnalysisResponse, error)
	getWorkRequest  func(context.Context, jmsutilssdk.GetWorkRequestRequest) (jmsutilssdk.GetWorkRequestResponse, error)
	requestRequests []jmsutilssdk.RequestJavaMigrationAnalysisRequest
	getRequests     []jmsutilssdk.GetJavaMigrationAnalysisRequest
	listRequests    []jmsutilssdk.ListJavaMigrationAnalysisRequest
	deleteRequests  []jmsutilssdk.DeleteJavaMigrationAnalysisRequest
	workReqRequests []jmsutilssdk.GetWorkRequestRequest
}

func (f *fakeJavaMigrationAnalysisOCIClient) RequestJavaMigrationAnalysis(
	ctx context.Context,
	request jmsutilssdk.RequestJavaMigrationAnalysisRequest,
) (jmsutilssdk.RequestJavaMigrationAnalysisResponse, error) {
	f.requestRequests = append(f.requestRequests, request)
	if f.request == nil {
		return jmsutilssdk.RequestJavaMigrationAnalysisResponse{}, fmt.Errorf("unexpected RequestJavaMigrationAnalysis call")
	}
	return f.request(ctx, request)
}

func (f *fakeJavaMigrationAnalysisOCIClient) GetJavaMigrationAnalysis(
	ctx context.Context,
	request jmsutilssdk.GetJavaMigrationAnalysisRequest,
) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error) {
	f.getRequests = append(f.getRequests, request)
	if f.get == nil {
		return jmsutilssdk.GetJavaMigrationAnalysisResponse{}, fmt.Errorf("unexpected GetJavaMigrationAnalysis call")
	}
	return f.get(ctx, request)
}

func (f *fakeJavaMigrationAnalysisOCIClient) ListJavaMigrationAnalysis(
	ctx context.Context,
	request jmsutilssdk.ListJavaMigrationAnalysisRequest,
) (jmsutilssdk.ListJavaMigrationAnalysisResponse, error) {
	f.listRequests = append(f.listRequests, request)
	if f.list == nil {
		return jmsutilssdk.ListJavaMigrationAnalysisResponse{}, fmt.Errorf("unexpected ListJavaMigrationAnalysis call")
	}
	return f.list(ctx, request)
}

func (f *fakeJavaMigrationAnalysisOCIClient) DeleteJavaMigrationAnalysis(
	ctx context.Context,
	request jmsutilssdk.DeleteJavaMigrationAnalysisRequest,
) (jmsutilssdk.DeleteJavaMigrationAnalysisResponse, error) {
	f.deleteRequests = append(f.deleteRequests, request)
	if f.delete == nil {
		return jmsutilssdk.DeleteJavaMigrationAnalysisResponse{}, fmt.Errorf("unexpected DeleteJavaMigrationAnalysis call")
	}
	return f.delete(ctx, request)
}

func (f *fakeJavaMigrationAnalysisOCIClient) GetWorkRequest(
	ctx context.Context,
	request jmsutilssdk.GetWorkRequestRequest,
) (jmsutilssdk.GetWorkRequestResponse, error) {
	f.workReqRequests = append(f.workReqRequests, request)
	if f.getWorkRequest == nil {
		return jmsutilssdk.GetWorkRequestResponse{}, fmt.Errorf("unexpected GetWorkRequest call")
	}
	return f.getWorkRequest(ctx, request)
}

func (f *fakeJavaMigrationAnalysisOCIClient) totalCalls() int {
	return len(f.requestRequests) + len(f.getRequests) + len(f.listRequests) + len(f.deleteRequests) + len(f.workReqRequests)
}

func makeJavaMigrationAnalysisResourceWithSpec() *jmsutilsv1beta1.JavaMigrationAnalysis {
	return &jmsutilsv1beta1.JavaMigrationAnalysis{
		Spec: jmsutilsv1beta1.JavaMigrationAnalysisSpec{
			CompartmentId:                       testCompartmentID,
			AnalysisProjectName:                 testProjectName,
			InputApplicationsObjectStoragePaths: []string{"oci://bucket/app.jar"},
			TargetJdkVersion:                    "17",
		},
	}
}

func assertJavaMigrationAnalysisCreateRequest(t *testing.T, request jmsutilssdk.RequestJavaMigrationAnalysisRequest) {
	t.Helper()
	if got := stringValue(request.CompartmentId); got != testCompartmentID {
		t.Fatalf("RequestJavaMigrationAnalysis compartmentId = %q, want %q", got, testCompartmentID)
	}
	if len(request.Targets) != 1 {
		t.Fatalf("RequestJavaMigrationAnalysis targets = %d, want 1", len(request.Targets))
	}
	target := request.Targets[0]
	if got := stringValue(target.AnalysisProjectName); got != testProjectName {
		t.Fatalf("RequestJavaMigrationAnalysis target.analysisProjectName = %q, want %q", got, testProjectName)
	}
	if got := target.InputApplicationsObjectStoragePaths; len(got) != 1 || got[0] != "oci://bucket/app.jar" {
		t.Fatalf("RequestJavaMigrationAnalysis target.inputApplicationsObjectStoragePaths = %#v", got)
	}
	if got := target.TargetJdkVersions; len(got) != 1 || got[0] != "17" {
		t.Fatalf("RequestJavaMigrationAnalysis target.targetJdkVersions = %#v", got)
	}
	if got := stringValue(request.OpcRetryToken); !strings.HasPrefix(got, "osok-jma-") {
		t.Fatalf("RequestJavaMigrationAnalysis opcRetryToken = %q, want stable osok token", got)
	}
}

func assertJavaMigrationAnalysisListRequest(t *testing.T, request jmsutilssdk.ListJavaMigrationAnalysisRequest) {
	t.Helper()
	if got := stringValue(request.CompartmentId); got != testCompartmentID {
		t.Fatalf("ListJavaMigrationAnalysis compartmentId = %q, want %q", got, testCompartmentID)
	}
	if got := stringValue(request.AnalysisProjectName); got != testProjectName {
		t.Fatalf("ListJavaMigrationAnalysis analysisProjectName = %q, want %q", got, testProjectName)
	}
}

func assertJavaMigrationAnalysisPendingCreate(
	t *testing.T,
	response servicemanager.OSOKResponse,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
	fake *fakeJavaMigrationAnalysisOCIClient,
) {
	t.Helper()
	if !response.IsSuccessful || !response.ShouldRequeue || response.RequeueDuration != javaMigrationAnalysisPollInterval {
		t.Fatalf("CreateOrUpdate() response = %+v, want pending create requeue", response)
	}
	if got := len(fake.listRequests); got != 1 {
		t.Fatalf("list calls = %d, want 1 pre-create bind lookup", got)
	}
	if got := len(fake.requestRequests); got != 1 {
		t.Fatalf("request calls = %d, want 1 create request", got)
	}
	if got := resource.Status.WorkRequestId; got != testWorkRequestID {
		t.Fatalf("status.workRequestId = %q, want %q", got, testWorkRequestID)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "create-request" {
		t.Fatalf("status.status.opcRequestId = %q, want create-request", got)
	}
}

func makeSDKJavaMigrationAnalysis(id string) jmsutilssdk.JavaMigrationAnalysis {
	return jmsutilssdk.JavaMigrationAnalysis{
		Id:                                  common.String(id),
		WorkRequestId:                       common.String(testWorkRequestID),
		CompartmentId:                       common.String(testCompartmentID),
		AnalysisProjectName:                 common.String(testProjectName),
		InputApplicationsObjectStoragePaths: []string{"oci://bucket/app.jar"},
		TargetJdkVersion:                    common.String("17"),
		NamespaceName:                       common.String("namespace"),
		BucketName:                          common.String("bucket"),
		AnalysisResultObjectStoragePath:     common.String("oci://bucket/results"),
		AnalysisResultFiles:                 []string{"summary.json"},
		TimeCreated:                         sdkTime("2026-05-19T10:00:00Z"),
		TimeStarted:                         sdkTime("2026-05-19T10:01:00Z"),
		TimeFinished:                        sdkTime("2026-05-19T10:05:00Z"),
		CreatedBy:                           &jmsutilssdk.Principal{Id: common.String("ocid1.user.oc1..creator"), DisplayName: common.String("creator")},
		Metadata:                            common.String("metadata"),
	}
}

func makeSDKJavaMigrationAnalysisSummary(id, compartmentID, projectName string) jmsutilssdk.JavaMigrationAnalysisSummary {
	return jmsutilssdk.JavaMigrationAnalysisSummary{
		Id:                                  common.String(id),
		CompartmentId:                       common.String(compartmentID),
		AnalysisProjectName:                 common.String(projectName),
		InputApplicationsObjectStoragePaths: []string{"oci://bucket/app.jar"},
		TargetJdkVersion:                    common.String("17"),
	}
}

func sdkTime(value string) *common.SDKTime {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return &common.SDKTime{Time: parsed}
}

func requireLatestCondition(
	t *testing.T,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
	want shared.OSOKConditionType,
) {
	t.Helper()
	if len(resource.Status.OsokStatus.Conditions) == 0 {
		t.Fatalf("conditions empty, want latest %s", want)
	}
	got := resource.Status.OsokStatus.Conditions[len(resource.Status.OsokStatus.Conditions)-1].Type
	if got != want {
		t.Fatalf("latest condition = %s, want %s", got, want)
	}
}
