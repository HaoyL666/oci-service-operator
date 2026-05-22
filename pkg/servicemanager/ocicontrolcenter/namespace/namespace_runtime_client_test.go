/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package namespace

import (
	"context"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	ocicontrolcentersdk "github.com/oracle/oci-go-sdk/v65/ocicontrolcenter"
	ocicontrolcenterv1beta1 "github.com/oracle/oci-service-operator/api/ocicontrolcenter/v1beta1"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testNamespaceTenancyID = "ocid1.tenancy.oc1..namespace"
	testNamespaceName      = "oci_computeagent"
)

func TestNamespaceCreateOrUpdateBindsAnnotatedNamespaceAcrossPages(t *testing.T) {
	t.Parallel()

	var requests []ocicontrolcentersdk.ListNamespacesRequest
	client := newNamespaceServiceClientWithPages(t, []ocicontrolcentersdk.ListNamespacesResponse{
		namespaceListResponse("request-1", "next", "oci_blockstorage"),
		namespaceListResponse("request-2", "", testNamespaceName),
	}, &requests)
	resource := testNamespaceResource("namespace-sample")
	resource.Annotations = map[string]string{namespaceNameAnnotation: testNamespaceName}

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() IsSuccessful = false, want true")
	}
	requireNamespaceListRequest(t, requests, 0, testNamespaceTenancyID, "")
	requireNamespaceListRequest(t, requests, 1, testNamespaceTenancyID, "next")
	if resource.Status.NamespaceName != testNamespaceName {
		t.Fatalf("status.namespaceName = %q, want %q", resource.Status.NamespaceName, testNamespaceName)
	}
	if resource.Status.OsokStatus.OpcRequestID != "request-2" {
		t.Fatalf("status opcRequestId = %q, want request-2", resource.Status.OsokStatus.OpcRequestID)
	}
	requireNamespaceCondition(t, resource, shared.Active)
}

func TestNamespaceCreateOrUpdateUsesRecordedStatusNameForNoOpReconcile(t *testing.T) {
	t.Parallel()

	var requests []ocicontrolcentersdk.ListNamespacesRequest
	client := newNamespaceServiceClientWithPages(t, []ocicontrolcentersdk.ListNamespacesResponse{
		namespaceListResponse("request-1", "", "metadata-name", testNamespaceName),
	}, &requests)
	resource := testNamespaceResource("metadata-name")
	resource.Status.NamespaceName = testNamespaceName

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() IsSuccessful = false, want true")
	}
	if resource.Status.NamespaceName != testNamespaceName {
		t.Fatalf("status.namespaceName = %q, want recorded %q", resource.Status.NamespaceName, testNamespaceName)
	}
	requireNamespaceListRequest(t, requests, 0, testNamespaceTenancyID, "")
	requireNamespaceCondition(t, resource, shared.Active)
}

func TestNamespaceCreateOrUpdateRejectsAnnotationDriftBeforeOCI(t *testing.T) {
	t.Parallel()

	listCalls := 0
	client := newNamespaceRuntimeClientForTest(fakeNamespaceProvider{tenancyID: testNamespaceTenancyID},
		func(context.Context, ocicontrolcentersdk.ListNamespacesRequest) (ocicontrolcentersdk.ListNamespacesResponse, error) {
			listCalls++
			return ocicontrolcentersdk.ListNamespacesResponse{}, nil
		})
	resource := testNamespaceResource("namespace-sample")
	resource.Status.NamespaceName = testNamespaceName
	resource.Annotations = map[string]string{namespaceNameAnnotation: "oci_changed"}

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want annotation drift error")
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() IsSuccessful = true, want false")
	}
	if listCalls != 0 {
		t.Fatalf("ListNamespaces() calls = %d, want 0", listCalls)
	}
	if !strings.Contains(err.Error(), namespaceNameAnnotation) {
		t.Fatalf("CreateOrUpdate() error = %v, want annotation name", err)
	}
	requireNamespaceCondition(t, resource, shared.Failed)
}

func TestNamespaceCreateOrUpdateRecordsListErrorRequestID(t *testing.T) {
	t.Parallel()

	client := newNamespaceRuntimeClientForTest(fakeNamespaceProvider{tenancyID: testNamespaceTenancyID},
		func(context.Context, ocicontrolcentersdk.ListNamespacesRequest) (ocicontrolcentersdk.ListNamespacesResponse, error) {
			return ocicontrolcentersdk.ListNamespacesResponse{}, namespaceRequestIDError{requestID: "opc-error"}
		})
	resource := testNamespaceResource(testNamespaceName)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want list error")
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() IsSuccessful = true, want false")
	}
	if resource.Status.OsokStatus.OpcRequestID != "opc-error" {
		t.Fatalf("status opcRequestId = %q, want opc-error", resource.Status.OsokStatus.OpcRequestID)
	}
	requireNamespaceCondition(t, resource, shared.Failed)
}

func TestNamespaceCreateOrUpdateRestoresActiveAfterListFailureWithUnchangedMessage(t *testing.T) {
	t.Parallel()

	calls := 0
	client := newNamespaceRuntimeClientForTest(fakeNamespaceProvider{tenancyID: testNamespaceTenancyID},
		func(context.Context, ocicontrolcentersdk.ListNamespacesRequest) (ocicontrolcentersdk.ListNamespacesResponse, error) {
			calls++
			switch calls {
			case 1:
				return namespaceListResponse("request-1", "", testNamespaceName), nil
			case 2:
				return ocicontrolcentersdk.ListNamespacesResponse{}, namespaceRequestIDError{requestID: "opc-error"}
			case 3:
				return namespaceListResponse("request-3", "", testNamespaceName), nil
			default:
				t.Fatalf("unexpected ListNamespaces() call %d", calls)
				return ocicontrolcentersdk.ListNamespacesResponse{}, nil
			}
		})
	resource := testNamespaceResource(testNamespaceName)

	if _, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{}); err != nil {
		t.Fatalf("initial CreateOrUpdate() error = %v", err)
	}
	requireNamespaceCondition(t, resource, shared.Active)

	if _, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{}); err == nil {
		t.Fatal("second CreateOrUpdate() error = nil, want list error")
	}
	requireNamespaceCondition(t, resource, shared.Failed)

	if _, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{}); err != nil {
		t.Fatalf("recovery CreateOrUpdate() error = %v", err)
	}
	requireNamespaceCondition(t, resource, shared.Active)
	if resource.Status.OsokStatus.OpcRequestID != "request-3" {
		t.Fatalf("status opcRequestId = %q, want request-3", resource.Status.OsokStatus.OpcRequestID)
	}
}

func TestNamespaceCreateOrUpdateScansAllPagesBeforeReportingMissing(t *testing.T) {
	t.Parallel()

	var requests []ocicontrolcentersdk.ListNamespacesRequest
	client := newNamespaceServiceClientWithPages(t, []ocicontrolcentersdk.ListNamespacesResponse{
		namespaceListResponse("request-1", "next", "oci_blockstorage"),
		namespaceListResponse("request-2", "", "oci_database"),
	}, &requests)
	resource := testNamespaceResource(testNamespaceName)

	_, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want missing namespace")
	}
	requireNamespaceListRequest(t, requests, 0, testNamespaceTenancyID, "")
	requireNamespaceListRequest(t, requests, 1, testNamespaceTenancyID, "next")
	if !strings.Contains(err.Error(), testNamespaceName) {
		t.Fatalf("CreateOrUpdate() error = %v, want target namespace", err)
	}
	requireNamespaceCondition(t, resource, shared.Failed)
}

func TestNamespaceCreateOrUpdateRejectsDuplicateMatchesAcrossPages(t *testing.T) {
	t.Parallel()

	var requests []ocicontrolcentersdk.ListNamespacesRequest
	client := newNamespaceServiceClientWithPages(t, []ocicontrolcentersdk.ListNamespacesResponse{
		namespaceListResponse("request-1", "next", testNamespaceName),
		namespaceListResponse("request-2", "", testNamespaceName),
	}, &requests)
	resource := testNamespaceResource(testNamespaceName)

	_, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want duplicate match error")
	}
	if !strings.Contains(err.Error(), "multiple matches") {
		t.Fatalf("CreateOrUpdate() error = %v, want multiple matches", err)
	}
	requireNamespaceListRequest(t, requests, 1, testNamespaceTenancyID, "next")
	requireNamespaceCondition(t, resource, shared.Failed)
}

func TestNamespaceCreateOrUpdateRequiresProviderTenancy(t *testing.T) {
	t.Parallel()

	listCalls := 0
	client := newNamespaceRuntimeClientForTest(fakeNamespaceProvider{err: errors.New("missing tenancy")},
		func(context.Context, ocicontrolcentersdk.ListNamespacesRequest) (ocicontrolcentersdk.ListNamespacesResponse, error) {
			listCalls++
			return ocicontrolcentersdk.ListNamespacesResponse{}, nil
		})
	resource := testNamespaceResource(testNamespaceName)

	_, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want tenancy error")
	}
	if listCalls != 0 {
		t.Fatalf("ListNamespaces() calls = %d, want 0", listCalls)
	}
	if !strings.Contains(err.Error(), "tenancy OCID") {
		t.Fatalf("CreateOrUpdate() error = %v, want tenancy OCID context", err)
	}
	requireNamespaceCondition(t, resource, shared.Failed)
}

func TestNamespaceDeleteReleasesReadOnlyResourceWithoutOCIList(t *testing.T) {
	t.Parallel()

	listCalls := 0
	client := newNamespaceRuntimeClientForTest(fakeNamespaceProvider{tenancyID: testNamespaceTenancyID},
		func(context.Context, ocicontrolcentersdk.ListNamespacesRequest) (ocicontrolcentersdk.ListNamespacesResponse, error) {
			listCalls++
			return ocicontrolcentersdk.ListNamespacesResponse{}, nil
		})
	resource := testNamespaceResource(testNamespaceName)
	resource.Status.NamespaceName = testNamespaceName

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatalf("Delete() deleted = false, want true")
	}
	if listCalls != 0 {
		t.Fatalf("ListNamespaces() calls = %d, want 0", listCalls)
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("status.deletedAt = nil, want timestamp")
	}
	if !strings.Contains(resource.Status.OsokStatus.Message, "read-only") {
		t.Fatalf("status.message = %q, want read-only context", resource.Status.OsokStatus.Message)
	}
	requireNamespaceCondition(t, resource, shared.Terminating)
}

func newNamespaceServiceClientWithPages(
	t *testing.T,
	pages []ocicontrolcentersdk.ListNamespacesResponse,
	requests *[]ocicontrolcentersdk.ListNamespacesRequest,
) NamespaceServiceClient {
	t.Helper()

	next := 0
	return newNamespaceRuntimeClientForTest(fakeNamespaceProvider{tenancyID: testNamespaceTenancyID},
		func(_ context.Context, request ocicontrolcentersdk.ListNamespacesRequest) (ocicontrolcentersdk.ListNamespacesResponse, error) {
			*requests = append(*requests, request)
			if next >= len(pages) {
				t.Fatalf("unexpected ListNamespaces() call %d", next+1)
			}
			response := pages[next]
			next++
			return response, nil
		})
}

func namespaceListResponse(requestID string, nextPage string, names ...string) ocicontrolcentersdk.ListNamespacesResponse {
	items := make([]ocicontrolcentersdk.NamespaceSummary, 0, len(names))
	for _, name := range names {
		items = append(items, ocicontrolcentersdk.NamespaceSummary{NamespaceName: common.String(name)})
	}
	response := ocicontrolcentersdk.ListNamespacesResponse{
		NamespaceCollection: ocicontrolcentersdk.NamespaceCollection{Items: items},
		OpcRequestId:        common.String(requestID),
	}
	if nextPage != "" {
		response.OpcNextPage = common.String(nextPage)
	}
	return response
}

func testNamespaceResource(name string) *ocicontrolcenterv1beta1.Namespace {
	return &ocicontrolcenterv1beta1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func requireNamespaceListRequest(
	t *testing.T,
	requests []ocicontrolcentersdk.ListNamespacesRequest,
	index int,
	wantCompartmentID string,
	wantPage string,
) {
	t.Helper()

	if len(requests) <= index {
		t.Fatalf("ListNamespaces() calls = %d, want request %d", len(requests), index+1)
	}
	request := requests[index]
	if got := namespaceStringValue(request.CompartmentId); got != wantCompartmentID {
		t.Fatalf("request[%d].CompartmentId = %q, want %q", index, got, wantCompartmentID)
	}
	if got := namespaceStringValue(request.Page); got != wantPage {
		t.Fatalf("request[%d].Page = %q, want %q", index, got, wantPage)
	}
}

func requireNamespaceCondition(
	t *testing.T,
	resource *ocicontrolcenterv1beta1.Namespace,
	want shared.OSOKConditionType,
) {
	t.Helper()

	conditions := resource.Status.OsokStatus.Conditions
	if len(conditions) == 0 {
		t.Fatalf("status.conditions is empty, want %s", want)
	}
	if got := conditions[len(conditions)-1].Type; got != want {
		t.Fatalf("last condition = %s, want %s", got, want)
	}
	if got := resource.Status.OsokStatus.Reason; got != string(want) {
		t.Fatalf("status.reason = %q, want %q", got, want)
	}
}

type fakeNamespaceProvider struct {
	tenancyID string
	err       error
}

func (p fakeNamespaceProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	return nil, nil
}

func (p fakeNamespaceProvider) KeyID() (string, error) {
	return "", nil
}

func (p fakeNamespaceProvider) TenancyOCID() (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.tenancyID, nil
}

func (p fakeNamespaceProvider) UserOCID() (string, error) {
	return "", nil
}

func (p fakeNamespaceProvider) KeyFingerprint() (string, error) {
	return "", nil
}

func (p fakeNamespaceProvider) Region() (string, error) {
	return "", nil
}

func (p fakeNamespaceProvider) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{}, nil
}

type namespaceRequestIDError struct {
	requestID string
}

func (e namespaceRequestIDError) Error() string {
	return "list namespace failed"
}

func (e namespaceRequestIDError) GetOpcRequestID() string {
	return e.requestID
}
