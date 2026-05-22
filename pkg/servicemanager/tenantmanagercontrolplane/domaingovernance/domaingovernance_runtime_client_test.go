/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package domaingovernance

import (
	"context"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	tenantmanagercontrolplanesdk "github.com/oracle/oci-go-sdk/v65/tenantmanagercontrolplane"
	tenantmanagercontrolplanev1beta1 "github.com/oracle/oci-service-operator/api/tenantmanagercontrolplane/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testDomainGovernanceID          = "ocid1.domaingovernance.oc1..example"
	testOtherDomainGovernanceID     = "ocid1.domaingovernance.oc1..other"
	testDomainGovernanceCompartment = "ocid1.tenancy.oc1..example"
	testOtherCompartmentID          = "ocid1.tenancy.oc1..other"
	testDomainID                    = "ocid1.domain.oc1..example"
	testOtherDomainID               = "ocid1.domain.oc1..other"
	testOnsTopicID                  = "ocid1.onstopic.oc1..topic"
	testOtherOnsTopicID             = "ocid1.onstopic.oc1..other"
	testOnsSubscriptionID           = "ocid1.onssubscription.oc1..subscription"
	testOtherOnsSubscriptionID      = "ocid1.onssubscription.oc1..other"
	testSubscriptionEmail           = "admin@example.com"
	testUpdatedSubscriptionEmail    = "updated@example.com"
)

type fakeDomainGovernanceRuntimeOCIClient struct {
	createFn func(context.Context, tenantmanagercontrolplanesdk.CreateDomainGovernanceRequest) (tenantmanagercontrolplanesdk.CreateDomainGovernanceResponse, error)
	getFn    func(context.Context, tenantmanagercontrolplanesdk.GetDomainGovernanceRequest) (tenantmanagercontrolplanesdk.GetDomainGovernanceResponse, error)
	listFn   func(context.Context, tenantmanagercontrolplanesdk.ListDomainGovernancesRequest) (tenantmanagercontrolplanesdk.ListDomainGovernancesResponse, error)
	updateFn func(context.Context, tenantmanagercontrolplanesdk.UpdateDomainGovernanceRequest) (tenantmanagercontrolplanesdk.UpdateDomainGovernanceResponse, error)
	deleteFn func(context.Context, tenantmanagercontrolplanesdk.DeleteDomainGovernanceRequest) (tenantmanagercontrolplanesdk.DeleteDomainGovernanceResponse, error)

	createRequests []tenantmanagercontrolplanesdk.CreateDomainGovernanceRequest
	getRequests    []tenantmanagercontrolplanesdk.GetDomainGovernanceRequest
	listRequests   []tenantmanagercontrolplanesdk.ListDomainGovernancesRequest
	updateRequests []tenantmanagercontrolplanesdk.UpdateDomainGovernanceRequest
	deleteRequests []tenantmanagercontrolplanesdk.DeleteDomainGovernanceRequest
}

func (f *fakeDomainGovernanceRuntimeOCIClient) CreateDomainGovernance(
	ctx context.Context,
	req tenantmanagercontrolplanesdk.CreateDomainGovernanceRequest,
) (tenantmanagercontrolplanesdk.CreateDomainGovernanceResponse, error) {
	f.createRequests = append(f.createRequests, req)
	if f.createFn != nil {
		return f.createFn(ctx, req)
	}
	return tenantmanagercontrolplanesdk.CreateDomainGovernanceResponse{}, nil
}

func (f *fakeDomainGovernanceRuntimeOCIClient) GetDomainGovernance(
	ctx context.Context,
	req tenantmanagercontrolplanesdk.GetDomainGovernanceRequest,
) (tenantmanagercontrolplanesdk.GetDomainGovernanceResponse, error) {
	f.getRequests = append(f.getRequests, req)
	if f.getFn != nil {
		return f.getFn(ctx, req)
	}
	return tenantmanagercontrolplanesdk.GetDomainGovernanceResponse{}, nil
}

func (f *fakeDomainGovernanceRuntimeOCIClient) ListDomainGovernances(
	ctx context.Context,
	req tenantmanagercontrolplanesdk.ListDomainGovernancesRequest,
) (tenantmanagercontrolplanesdk.ListDomainGovernancesResponse, error) {
	f.listRequests = append(f.listRequests, req)
	if f.listFn != nil {
		return f.listFn(ctx, req)
	}
	return tenantmanagercontrolplanesdk.ListDomainGovernancesResponse{}, nil
}

func (f *fakeDomainGovernanceRuntimeOCIClient) UpdateDomainGovernance(
	ctx context.Context,
	req tenantmanagercontrolplanesdk.UpdateDomainGovernanceRequest,
) (tenantmanagercontrolplanesdk.UpdateDomainGovernanceResponse, error) {
	f.updateRequests = append(f.updateRequests, req)
	if f.updateFn != nil {
		return f.updateFn(ctx, req)
	}
	return tenantmanagercontrolplanesdk.UpdateDomainGovernanceResponse{}, nil
}

func (f *fakeDomainGovernanceRuntimeOCIClient) DeleteDomainGovernance(
	ctx context.Context,
	req tenantmanagercontrolplanesdk.DeleteDomainGovernanceRequest,
) (tenantmanagercontrolplanesdk.DeleteDomainGovernanceResponse, error) {
	f.deleteRequests = append(f.deleteRequests, req)
	if f.deleteFn != nil {
		return f.deleteFn(ctx, req)
	}
	return tenantmanagercontrolplanesdk.DeleteDomainGovernanceResponse{}, nil
}

func TestDomainGovernanceCreateOrUpdateCreatesWithoutFollowUpReadAndTreatsInactiveAsSuccess(t *testing.T) {
	t.Parallel()

	resource := newDomainGovernanceResource()
	fake := &fakeDomainGovernanceRuntimeOCIClient{
		listFn: func(_ context.Context, req tenantmanagercontrolplanesdk.ListDomainGovernancesRequest) (tenantmanagercontrolplanesdk.ListDomainGovernancesResponse, error) {
			requireStringPtr(t, "ListDomainGovernancesRequest.CompartmentId", req.CompartmentId, testDomainGovernanceCompartment)
			requireStringPtr(t, "ListDomainGovernancesRequest.DomainId", req.DomainId, testDomainID)
			return tenantmanagercontrolplanesdk.ListDomainGovernancesResponse{
				DomainGovernanceCollection: tenantmanagercontrolplanesdk.DomainGovernanceCollection{},
			}, nil
		},
		createFn: func(_ context.Context, req tenantmanagercontrolplanesdk.CreateDomainGovernanceRequest) (tenantmanagercontrolplanesdk.CreateDomainGovernanceResponse, error) {
			requireStringPtr(t, "CreateDomainGovernanceRequest.CompartmentId", req.CompartmentId, testDomainGovernanceCompartment)
			requireStringPtr(t, "CreateDomainGovernanceRequest.DomainId", req.DomainId, testDomainID)
			requireStringPtr(t, "CreateDomainGovernanceRequest.SubscriptionEmail", req.SubscriptionEmail, testSubscriptionEmail)
			requireStringPtr(t, "CreateDomainGovernanceRequest.OnsTopicId", req.OnsTopicId, testOnsTopicID)
			requireStringPtr(t, "CreateDomainGovernanceRequest.OnsSubscriptionId", req.OnsSubscriptionId, testOnsSubscriptionID)
			return tenantmanagercontrolplanesdk.CreateDomainGovernanceResponse{
				DomainGovernance: inactiveDomainGovernanceSDK(testDomainGovernanceID),
			}, nil
		},
		getFn: func(context.Context, tenantmanagercontrolplanesdk.GetDomainGovernanceRequest) (tenantmanagercontrolplanesdk.GetDomainGovernanceResponse, error) {
			t.Fatal("GetDomainGovernance() called, want direct-body create without follow-up read")
			return tenantmanagercontrolplanesdk.GetDomainGovernanceResponse{}, nil
		},
	}

	response, err := newTestDomainGovernanceClient(fake).CreateOrUpdate(
		context.Background(),
		resource,
		requestForDomainGovernance(resource),
	)
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %#v, want successful steady-state create", response)
	}
	if len(fake.createRequests) != 1 {
		t.Fatalf("CreateDomainGovernance calls = %d, want 1", len(fake.createRequests))
	}
	if len(fake.listRequests) != 1 {
		t.Fatalf("ListDomainGovernances calls = %d, want 1 pre-create reuse probe", len(fake.listRequests))
	}
	if len(fake.getRequests) != 0 || len(fake.updateRequests) != 0 {
		t.Fatalf("unexpected OCI calls: get=%d list=%d update=%d", len(fake.getRequests), len(fake.listRequests), len(fake.updateRequests))
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != testDomainGovernanceID {
		t.Fatalf("status.ocid = %q, want %q", got, testDomainGovernanceID)
	}
	if got := resource.Status.LifecycleState; got != string(tenantmanagercontrolplanesdk.DomainGovernanceLifecycleStateInactive) {
		t.Fatalf("status.lifecycleState = %q, want %q", got, tenantmanagercontrolplanesdk.DomainGovernanceLifecycleStateInactive)
	}
}

func TestDomainGovernanceCreateOrUpdateBindsExistingByPagedList(t *testing.T) {
	t.Parallel()

	resource := newDomainGovernanceResource()
	fake := &fakeDomainGovernanceRuntimeOCIClient{
		listFn: func(_ context.Context, req tenantmanagercontrolplanesdk.ListDomainGovernancesRequest) (tenantmanagercontrolplanesdk.ListDomainGovernancesResponse, error) {
			requireStringPtr(t, "ListDomainGovernancesRequest.CompartmentId", req.CompartmentId, testDomainGovernanceCompartment)
			requireStringPtr(t, "ListDomainGovernancesRequest.DomainId", req.DomainId, testDomainID)
			switch page := stringPtrValue(req.Page); page {
			case "":
				return tenantmanagercontrolplanesdk.ListDomainGovernancesResponse{
					DomainGovernanceCollection: tenantmanagercontrolplanesdk.DomainGovernanceCollection{},
					OpcNextPage:                common.String("page-2"),
				}, nil
			case "page-2":
				return tenantmanagercontrolplanesdk.ListDomainGovernancesResponse{
					DomainGovernanceCollection: tenantmanagercontrolplanesdk.DomainGovernanceCollection{
						Items: []tenantmanagercontrolplanesdk.DomainGovernanceSummary{
							activeDomainGovernanceSummarySDK(testDomainGovernanceID),
						},
					},
				}, nil
			default:
				t.Fatalf("unexpected list page %q", page)
				return tenantmanagercontrolplanesdk.ListDomainGovernancesResponse{}, nil
			}
		},
		getFn: func(_ context.Context, req tenantmanagercontrolplanesdk.GetDomainGovernanceRequest) (tenantmanagercontrolplanesdk.GetDomainGovernanceResponse, error) {
			requireStringPtr(t, "GetDomainGovernanceRequest.DomainGovernanceId", req.DomainGovernanceId, testDomainGovernanceID)
			return tenantmanagercontrolplanesdk.GetDomainGovernanceResponse{
				DomainGovernance: activeDomainGovernanceSDK(testDomainGovernanceID),
			}, nil
		},
	}

	response, err := newTestDomainGovernanceClient(fake).CreateOrUpdate(
		context.Background(),
		resource,
		requestForDomainGovernance(resource),
	)
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %#v, want successful bind without requeue", response)
	}
	if len(fake.listRequests) != 2 {
		t.Fatalf("ListDomainGovernances calls = %d, want 2 paged lookups", len(fake.listRequests))
	}
	if len(fake.getRequests) != 1 {
		t.Fatalf("GetDomainGovernance calls = %d, want 1 live read after list bind", len(fake.getRequests))
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != testDomainGovernanceID {
		t.Fatalf("status.ocid = %q, want %q", got, testDomainGovernanceID)
	}
}

func TestDomainGovernanceCreateOrUpdateUpdatesMutableFieldsWithoutFollowUpRead(t *testing.T) {
	t.Parallel()

	resource := trackedDomainGovernanceResource()
	resource.Spec.SubscriptionEmail = testUpdatedSubscriptionEmail
	resource.Spec.IsGovernanceEnabled = true
	resource.Spec.FreeformTags = map[string]string{"managed-by": "osok"}

	fake := &fakeDomainGovernanceRuntimeOCIClient{
		getFn: func(_ context.Context, req tenantmanagercontrolplanesdk.GetDomainGovernanceRequest) (tenantmanagercontrolplanesdk.GetDomainGovernanceResponse, error) {
			requireStringPtr(t, "GetDomainGovernanceRequest.DomainGovernanceId", req.DomainGovernanceId, testDomainGovernanceID)
			return tenantmanagercontrolplanesdk.GetDomainGovernanceResponse{
				DomainGovernance: activeDomainGovernanceSDK(testDomainGovernanceID),
			}, nil
		},
		updateFn: func(_ context.Context, req tenantmanagercontrolplanesdk.UpdateDomainGovernanceRequest) (tenantmanagercontrolplanesdk.UpdateDomainGovernanceResponse, error) {
			requireStringPtr(t, "UpdateDomainGovernanceRequest.DomainGovernanceId", req.DomainGovernanceId, testDomainGovernanceID)
			requireStringPtr(t, "UpdateDomainGovernanceRequest.SubscriptionEmail", req.SubscriptionEmail, testUpdatedSubscriptionEmail)
			requireBoolPtr(t, "UpdateDomainGovernanceRequest.IsGovernanceEnabled", req.IsGovernanceEnabled, true)
			if got := req.FreeformTags["managed-by"]; got != "osok" {
				t.Fatalf("UpdateDomainGovernanceRequest.FreeformTags[managed-by] = %q, want %q", got, "osok")
			}
			updated := activeDomainGovernanceSDK(testDomainGovernanceID)
			updated.SubscriptionEmail = common.String(testUpdatedSubscriptionEmail)
			updated.IsGovernanceEnabled = common.Bool(true)
			updated.FreeformTags = map[string]string{"managed-by": "osok"}
			return tenantmanagercontrolplanesdk.UpdateDomainGovernanceResponse{
				DomainGovernance: updated,
			}, nil
		},
	}

	response, err := newTestDomainGovernanceClient(fake).CreateOrUpdate(
		context.Background(),
		resource,
		requestForDomainGovernance(resource),
	)
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %#v, want successful update without requeue", response)
	}
	if len(fake.getRequests) != 1 {
		t.Fatalf("GetDomainGovernance calls = %d, want 1 current-state read before update", len(fake.getRequests))
	}
	if len(fake.updateRequests) != 1 {
		t.Fatalf("UpdateDomainGovernance calls = %d, want 1", len(fake.updateRequests))
	}
	if got := resource.Status.SubscriptionEmail; got != testUpdatedSubscriptionEmail {
		t.Fatalf("status.subscriptionEmail = %q, want %q", got, testUpdatedSubscriptionEmail)
	}
	if !resource.Status.IsGovernanceEnabled {
		t.Fatal("status.isGovernanceEnabled = false, want true")
	}
}

func TestDomainGovernanceCreateOrUpdateRejectsTrackedCreateOnlyDrift(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		mutate    func(*tenantmanagercontrolplanev1beta1.DomainGovernance)
		wantError string
	}{
		{
			name: "compartmentId",
			mutate: func(resource *tenantmanagercontrolplanev1beta1.DomainGovernance) {
				resource.Spec.CompartmentId = testOtherCompartmentID
			},
			wantError: "DomainGovernance formal semantics require replacement when compartmentId changes",
		},
		{
			name: "onsTopicId",
			mutate: func(resource *tenantmanagercontrolplanev1beta1.DomainGovernance) {
				resource.Spec.OnsTopicId = testOtherOnsTopicID
			},
			wantError: "DomainGovernance formal semantics require replacement when onsTopicId changes",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resource := trackedDomainGovernanceResource()
			tc.mutate(resource)
			fake := &fakeDomainGovernanceRuntimeOCIClient{
				getFn: func(_ context.Context, req tenantmanagercontrolplanesdk.GetDomainGovernanceRequest) (tenantmanagercontrolplanesdk.GetDomainGovernanceResponse, error) {
					requireStringPtr(t, "GetDomainGovernanceRequest.DomainGovernanceId", req.DomainGovernanceId, testDomainGovernanceID)
					return tenantmanagercontrolplanesdk.GetDomainGovernanceResponse{
						DomainGovernance: activeDomainGovernanceSDK(testDomainGovernanceID),
					}, nil
				},
			}

			response, err := newTestDomainGovernanceClient(fake).CreateOrUpdate(
				context.Background(),
				resource,
				requestForDomainGovernance(resource),
			)
			if err == nil || err.Error() != tc.wantError {
				t.Fatalf("CreateOrUpdate() error = %v, want %q", err, tc.wantError)
			}
			if response.IsSuccessful {
				t.Fatalf("CreateOrUpdate() response = %#v, want unsuccessful drift rejection", response)
			}
			if len(fake.updateRequests) != 0 {
				t.Fatalf("UpdateDomainGovernance calls = %d, want 0 after drift rejection", len(fake.updateRequests))
			}
		})
	}
}

func TestDomainGovernanceDeleteKeepsFinalizerUntilResourceDisappears(t *testing.T) {
	t.Parallel()

	resource := trackedDomainGovernanceResource()
	fake := &fakeDomainGovernanceRuntimeOCIClient{
		deleteFn: func(_ context.Context, req tenantmanagercontrolplanesdk.DeleteDomainGovernanceRequest) (tenantmanagercontrolplanesdk.DeleteDomainGovernanceResponse, error) {
			requireStringPtr(t, "DeleteDomainGovernanceRequest.DomainGovernanceId", req.DomainGovernanceId, testDomainGovernanceID)
			return tenantmanagercontrolplanesdk.DeleteDomainGovernanceResponse{}, nil
		},
		getFn: func(_ context.Context, req tenantmanagercontrolplanesdk.GetDomainGovernanceRequest) (tenantmanagercontrolplanesdk.GetDomainGovernanceResponse, error) {
			requireStringPtr(t, "GetDomainGovernanceRequest.DomainGovernanceId", req.DomainGovernanceId, testDomainGovernanceID)
			return tenantmanagercontrolplanesdk.GetDomainGovernanceResponse{
				DomainGovernance: inactiveDomainGovernanceSDK(testDomainGovernanceID),
			}, nil
		},
	}

	deleted, err := newTestDomainGovernanceClient(fake).Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted {
		t.Fatal("Delete() = true, want pending delete while resource is still readable")
	}
	if len(fake.deleteRequests) != 1 {
		t.Fatalf("DeleteDomainGovernance calls = %d, want 1", len(fake.deleteRequests))
	}
	if len(fake.getRequests) != 2 {
		t.Fatalf("GetDomainGovernance calls = %d, want 2 confirm-delete reads", len(fake.getRequests))
	}
	if resource.Status.OsokStatus.Async.Current == nil {
		t.Fatal("status.async.current = nil, want lifecycle delete tracker")
	}
	if resource.Status.OsokStatus.Async.Current.Phase != shared.OSOKAsyncPhaseDelete {
		t.Fatalf("status.async.current.phase = %q, want %q", resource.Status.OsokStatus.Async.Current.Phase, shared.OSOKAsyncPhaseDelete)
	}
	if resource.Status.OsokStatus.Message != domainGovernanceDeletePendingMessage {
		t.Fatalf("status.message = %q, want %q", resource.Status.OsokStatus.Message, domainGovernanceDeletePendingMessage)
	}
}

func newTestDomainGovernanceClient(fake *fakeDomainGovernanceRuntimeOCIClient) DomainGovernanceServiceClient {
	return newDomainGovernanceServiceClientWithClients(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("test")},
		fake,
	)
}

func newDomainGovernanceResource() *tenantmanagercontrolplanev1beta1.DomainGovernance {
	return &tenantmanagercontrolplanev1beta1.DomainGovernance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example",
			Namespace: "default",
		},
		Spec: tenantmanagercontrolplanev1beta1.DomainGovernanceSpec{
			CompartmentId:     testDomainGovernanceCompartment,
			DomainId:          testDomainID,
			SubscriptionEmail: testSubscriptionEmail,
			OnsTopicId:        testOnsTopicID,
			OnsSubscriptionId: testOnsSubscriptionID,
			FreeformTags:      map[string]string{"env": "test"},
		},
	}
}

func trackedDomainGovernanceResource() *tenantmanagercontrolplanev1beta1.DomainGovernance {
	resource := newDomainGovernanceResource()
	resource.Status = tenantmanagercontrolplanev1beta1.DomainGovernanceStatus{
		OsokStatus:          shared.OSOKStatus{Ocid: shared.OCID(testDomainGovernanceID)},
		Id:                  testDomainGovernanceID,
		OwnerId:             testDomainGovernanceCompartment,
		DomainId:            testDomainID,
		LifecycleState:      string(tenantmanagercontrolplanesdk.DomainGovernanceLifecycleStateActive),
		OnsTopicId:          testOnsTopicID,
		OnsSubscriptionId:   testOnsSubscriptionID,
		SubscriptionEmail:   testSubscriptionEmail,
		IsGovernanceEnabled: false,
		FreeformTags:        map[string]string{"env": "test"},
	}
	return resource
}

func requestForDomainGovernance(resource *tenantmanagercontrolplanev1beta1.DomainGovernance) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      resource.Name,
			Namespace: resource.Namespace,
		},
	}
}

func activeDomainGovernanceSDK(id string) tenantmanagercontrolplanesdk.DomainGovernance {
	return tenantmanagercontrolplanesdk.DomainGovernance{
		Id:                  common.String(id),
		OwnerId:             common.String(testDomainGovernanceCompartment),
		DomainId:            common.String(testDomainID),
		LifecycleState:      tenantmanagercontrolplanesdk.DomainGovernanceLifecycleStateActive,
		OnsTopicId:          common.String(testOnsTopicID),
		OnsSubscriptionId:   common.String(testOnsSubscriptionID),
		IsGovernanceEnabled: common.Bool(false),
		SubscriptionEmail:   common.String(testSubscriptionEmail),
		FreeformTags:        map[string]string{"env": "test"},
	}
}

func inactiveDomainGovernanceSDK(id string) tenantmanagercontrolplanesdk.DomainGovernance {
	domainGovernance := activeDomainGovernanceSDK(id)
	domainGovernance.LifecycleState = tenantmanagercontrolplanesdk.DomainGovernanceLifecycleStateInactive
	return domainGovernance
}

func activeDomainGovernanceSummarySDK(id string) tenantmanagercontrolplanesdk.DomainGovernanceSummary {
	return tenantmanagercontrolplanesdk.DomainGovernanceSummary{
		Id:                  common.String(id),
		OwnerId:             common.String(testDomainGovernanceCompartment),
		DomainId:            common.String(testDomainID),
		LifecycleState:      tenantmanagercontrolplanesdk.DomainGovernanceLifecycleStateActive,
		IsGovernanceEnabled: common.Bool(false),
		OnsTopicId:          common.String(testOnsTopicID),
		OnsSubscriptionId:   common.String(testOnsSubscriptionID),
		SubscriptionEmail:   common.String(testSubscriptionEmail),
		FreeformTags:        map[string]string{"env": "test"},
	}
}

func requireStringPtr(t *testing.T, label string, actual *string, want string) {
	t.Helper()
	if actual == nil {
		t.Fatalf("%s = nil, want %q", label, want)
	}
	if got := *actual; got != want {
		t.Fatalf("%s = %q, want %q", label, got, want)
	}
}

func requireBoolPtr(t *testing.T, label string, actual *bool, want bool) {
	t.Helper()
	if actual == nil {
		t.Fatalf("%s = nil, want %t", label, want)
	}
	if got := *actual; got != want {
		t.Fatalf("%s = %t, want %t", label, got, want)
	}
}
