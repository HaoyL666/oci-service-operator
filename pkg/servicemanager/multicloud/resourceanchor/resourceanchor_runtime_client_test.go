/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package resourceanchor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	multicloudsdk "github.com/oracle/oci-go-sdk/v65/multicloud"
	multicloudv1beta1 "github.com/oracle/oci-service-operator/api/multicloud/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil"
	"github.com/oracle/oci-service-operator/pkg/errorutil/errortest"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testResourceAnchorID           = "ocid1.resourceanchor.oc1..target"
	testOtherResourceAnchorID      = "ocid1.resourceanchor.oc1..other"
	testResourceAnchorSubscription = "ocid1.multicloudsubscription.oc1..sub"
	testResourceAnchorCompartment  = "ocid1.compartment.oc1..base"
)

type fakeResourceAnchorOCI struct {
	getCalls  []multicloudsdk.GetResourceAnchorRequest
	listCalls []multicloudsdk.ListResourceAnchorsRequest
	get       func(context.Context, multicloudsdk.GetResourceAnchorRequest) (multicloudsdk.GetResourceAnchorResponse, error)
	list      func(context.Context, multicloudsdk.ListResourceAnchorsRequest) (multicloudsdk.ListResourceAnchorsResponse, error)
}

func (f *fakeResourceAnchorOCI) GetResourceAnchor(ctx context.Context, request multicloudsdk.GetResourceAnchorRequest) (multicloudsdk.GetResourceAnchorResponse, error) {
	f.getCalls = append(f.getCalls, request)
	if f.get != nil {
		return f.get(ctx, request)
	}
	return multicloudsdk.GetResourceAnchorResponse{}, nil
}

func (f *fakeResourceAnchorOCI) ListResourceAnchors(ctx context.Context, request multicloudsdk.ListResourceAnchorsRequest) (multicloudsdk.ListResourceAnchorsResponse, error) {
	f.listCalls = append(f.listCalls, request)
	if f.list != nil {
		return f.list(ctx, request)
	}
	return multicloudsdk.ListResourceAnchorsResponse{}, nil
}

type fakeResourceAnchorDelegate struct {
	deleteCalled bool
	deleteResult bool
	deleteErr    error
}

func (f *fakeResourceAnchorDelegate) CreateOrUpdate(context.Context, *multicloudv1beta1.ResourceAnchor, ctrl.Request) (servicemanager.OSOKResponse, error) {
	return servicemanager.OSOKResponse{}, nil
}

func (f *fakeResourceAnchorDelegate) Delete(context.Context, *multicloudv1beta1.ResourceAnchor) (bool, error) {
	f.deleteCalled = true
	return f.deleteResult, f.deleteErr
}

func TestResourceAnchorCreateOrUpdateGetsByRecordedIdentity(t *testing.T) {
	fake := &fakeResourceAnchorOCI{
		get: successfulResourceAnchorGet(t),
	}
	client := testResourceAnchorClient(fake)
	resource := testResourceAnchorResource(map[string]string{
		resourceAnchorIDAnnotation:                      testResourceAnchorID,
		resourceAnchorSubscriptionIDAnnotation:          testResourceAnchorSubscription,
		resourceAnchorSubscriptionServiceNameAnnotation: "ORACLEDBATAZURE",
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %#v, want active success", response)
	}
	if len(fake.getCalls) != 1 || len(fake.listCalls) != 0 {
		t.Fatalf("OCI calls: get=%d list=%d, want one get only", len(fake.getCalls), len(fake.listCalls))
	}
	requireGetProjection(t, resource)
	requireCondition(t, resource, shared.Active)
}

func TestResourceAnchorCreateOrUpdateBindsFromLaterListPage(t *testing.T) {
	nextPage := "page-two"
	requestID := "opc-list-resource-anchor"
	var fake *fakeResourceAnchorOCI
	fake = &fakeResourceAnchorOCI{
		list: paginatedResourceAnchorList(t, func() int { return len(fake.listCalls) }, nextPage, requestID),
	}
	client := testResourceAnchorClient(fake)
	resource := testResourceAnchorResource(map[string]string{
		resourceAnchorDisplayNameAnnotation:   "target-anchor",
		resourceAnchorCompartmentIDAnnotation: testResourceAnchorCompartment,
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %#v, want successful active bind", response)
	}
	if len(fake.listCalls) != 2 {
		t.Fatalf("ListResourceAnchors calls = %d, want 2", len(fake.listCalls))
	}
	if resource.Status.Id != testResourceAnchorID {
		t.Fatalf("status.id = %q, want %q", resource.Status.Id, testResourceAnchorID)
	}
	if resource.Status.OsokStatus.OpcRequestID != requestID {
		t.Fatalf("status.status.opcRequestId = %q, want %q", resource.Status.OsokStatus.OpcRequestID, requestID)
	}
}

func TestResourceAnchorCreateOrUpdateRejectsMissingBindCriteriaBeforeOCI(t *testing.T) {
	fake := &fakeResourceAnchorOCI{}
	client := testResourceAnchorClient(fake)
	resource := testResourceAnchorResource(nil)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("CreateOrUpdate() error = %v, want read-only bind criteria error", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() response = %#v, want unsuccessful", response)
	}
	if len(fake.getCalls) != 0 || len(fake.listCalls) != 0 {
		t.Fatalf("OCI calls: get=%d list=%d, want none", len(fake.getCalls), len(fake.listCalls))
	}
	requireCondition(t, resource, shared.Failed)
}

func TestResourceAnchorCreateOrUpdateRejectsIdentityAnnotationDriftBeforeOCI(t *testing.T) {
	fake := &fakeResourceAnchorOCI{}
	client := testResourceAnchorClient(fake)
	resource := testResourceAnchorResource(map[string]string{
		resourceAnchorIDAnnotation: "ocid1.resourceanchor.oc1..changed",
	})
	resource.Status.OsokStatus.Ocid = shared.OCID(testResourceAnchorID)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil || !strings.Contains(err.Error(), "create-only identity annotation") {
		t.Fatalf("CreateOrUpdate() error = %v, want identity drift rejection", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() response = %#v, want unsuccessful", response)
	}
	if len(fake.getCalls) != 0 || len(fake.listCalls) != 0 {
		t.Fatalf("OCI calls: get=%d list=%d, want none", len(fake.getCalls), len(fake.listCalls))
	}
}

func TestResourceAnchorCreateOrUpdateRequeuesPendingLifecycle(t *testing.T) {
	fake := &fakeResourceAnchorOCI{
		get: func(context.Context, multicloudsdk.GetResourceAnchorRequest) (multicloudsdk.GetResourceAnchorResponse, error) {
			return multicloudsdk.GetResourceAnchorResponse{
				ResourceAnchor: testResourceAnchor(testResourceAnchorID, "target-anchor", multicloudsdk.ResourceAnchorLifecycleStateCreating),
			}, nil
		},
	}
	client := testResourceAnchorClient(fake)
	resource := testResourceAnchorResource(map[string]string{
		resourceAnchorIDAnnotation:                      testResourceAnchorID,
		resourceAnchorSubscriptionIDAnnotation:          testResourceAnchorSubscription,
		resourceAnchorSubscriptionServiceNameAnnotation: "ORACLEDBATAZURE",
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || !response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %#v, want successful pending requeue", response)
	}
	if resource.Status.OsokStatus.Async.Current == nil {
		t.Fatal("status.status.async.current = nil, want lifecycle tracking")
	}
	if resource.Status.OsokStatus.Async.Current.Phase != shared.OSOKAsyncPhaseCreate {
		t.Fatalf("async phase = %q, want create", resource.Status.OsokStatus.Async.Current.Phase)
	}
	requireCondition(t, resource, shared.Provisioning)
}

func TestResourceAnchorCreateOrUpdateRecordsOCIErrorRequestID(t *testing.T) {
	fake := &fakeResourceAnchorOCI{
		get: func(context.Context, multicloudsdk.GetResourceAnchorRequest) (multicloudsdk.GetResourceAnchorResponse, error) {
			serviceErr := errortest.NewServiceError(404, errorutil.NotAuthorizedOrNotFound, "not authorized or not found")
			serviceErr.OpcRequestID = "opc-error-resource-anchor"
			return multicloudsdk.GetResourceAnchorResponse{}, serviceErr
		},
	}
	client := testResourceAnchorClient(fake)
	resource := testResourceAnchorResource(map[string]string{
		resourceAnchorIDAnnotation:                      testResourceAnchorID,
		resourceAnchorSubscriptionIDAnnotation:          testResourceAnchorSubscription,
		resourceAnchorSubscriptionServiceNameAnnotation: "ORACLEDBATAZURE",
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want OCI error")
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate() response = %#v, want unsuccessful", response)
	}
	if resource.Status.OsokStatus.OpcRequestID != "opc-error-resource-anchor" {
		t.Fatalf("status.status.opcRequestId = %q, want opc-error-resource-anchor", resource.Status.OsokStatus.OpcRequestID)
	}
	requireCondition(t, resource, shared.Failed)
}

func TestResourceAnchorDefaultDeleteReleasesFinalizerWhenSDKDeleteIsUnsupported(t *testing.T) {
	hooks := newResourceAnchorDefaultRuntimeHooks(multicloudsdk.OmhubResourceAnchorClient{})
	delegate := defaultResourceAnchorServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*multicloudv1beta1.ResourceAnchor](
			buildResourceAnchorGeneratedRuntimeConfig(&ResourceAnchorServiceManager{}, hooks),
		),
	}
	resource := testResourceAnchorResource(nil)
	resource.Status.OsokStatus.Ocid = shared.OCID(testResourceAnchorID)

	deleted, err := delegate.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() = false, want true for SDK read-only ResourceAnchor")
	}
	requireCondition(t, resource, shared.Terminating)
}

func TestResourceAnchorWrapperDelegatesDeleteWithoutOCIRead(t *testing.T) {
	fake := &fakeResourceAnchorOCI{}
	delegate := &fakeResourceAnchorDelegate{deleteResult: true}
	client := resourceAnchorReadOnlyClient{
		delegate: delegate,
		get:      fake.GetResourceAnchor,
		list:     fake.ListResourceAnchors,
	}

	deleted, err := client.Delete(context.Background(), testResourceAnchorResource(nil))
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted || !delegate.deleteCalled {
		t.Fatalf("Delete() = (%t, called=%t), want delegated true", deleted, delegate.deleteCalled)
	}
	if len(fake.getCalls) != 0 || len(fake.listCalls) != 0 {
		t.Fatalf("OCI calls: get=%d list=%d, want none", len(fake.getCalls), len(fake.listCalls))
	}
}

func successfulResourceAnchorGet(t *testing.T) resourceAnchorGetCall {
	t.Helper()
	return func(_ context.Context, request multicloudsdk.GetResourceAnchorRequest) (multicloudsdk.GetResourceAnchorResponse, error) {
		requireGetRequest(t, request)
		requestID := "opc-get-resource-anchor"
		return multicloudsdk.GetResourceAnchorResponse{
			ResourceAnchor: testResourceAnchor(testResourceAnchorID, "target-anchor", multicloudsdk.ResourceAnchorLifecycleStateActive),
			OpcRequestId:   &requestID,
		}, nil
	}
}

func requireGetRequest(t *testing.T, request multicloudsdk.GetResourceAnchorRequest) {
	t.Helper()
	if got := stringValue(request.ResourceAnchorId); got != testResourceAnchorID {
		t.Fatalf("ResourceAnchorId = %q, want %q", got, testResourceAnchorID)
	}
	if got := stringValue(request.SubscriptionId); got != testResourceAnchorSubscription {
		t.Fatalf("SubscriptionId = %q, want %q", got, testResourceAnchorSubscription)
	}
	if request.SubscriptionServiceName != multicloudsdk.GetResourceAnchorSubscriptionServiceNameOracledbatazure {
		t.Fatalf("SubscriptionServiceName = %q, want ORACLEDBATAZURE", request.SubscriptionServiceName)
	}
}

func requireGetProjection(t *testing.T, resource *multicloudv1beta1.ResourceAnchor) {
	t.Helper()
	if resource.Status.Id != testResourceAnchorID || string(resource.Status.OsokStatus.Ocid) != testResourceAnchorID {
		t.Fatalf("status identity = (%q, %q), want %q", resource.Status.Id, resource.Status.OsokStatus.Ocid, testResourceAnchorID)
	}
	if resource.Status.SubscriptionType != "ORACLEDBATAZURE" {
		t.Fatalf("status.subscriptionType = %q, want ORACLEDBATAZURE", resource.Status.SubscriptionType)
	}
	if resource.Status.CloudServiceProviderMetadataItem.CspResourceAnchorId != "azure-anchor-id" {
		t.Fatalf("status.cloudServiceProviderMetadataItem.cspResourceAnchorId = %q, want azure-anchor-id", resource.Status.CloudServiceProviderMetadataItem.CspResourceAnchorId)
	}
	if resource.Status.OsokStatus.OpcRequestID != "opc-get-resource-anchor" {
		t.Fatalf("status.status.opcRequestId = %q, want opc-get-resource-anchor", resource.Status.OsokStatus.OpcRequestID)
	}
}

func paginatedResourceAnchorList(t *testing.T, callNumber func() int, nextPage string, requestID string) resourceAnchorListCall {
	t.Helper()
	return func(_ context.Context, request multicloudsdk.ListResourceAnchorsRequest) (multicloudsdk.ListResourceAnchorsResponse, error) {
		switch callNumber() {
		case 1:
			requireNilListPage(t, request)
			return resourceAnchorListPage(testOtherResourceAnchorID, "other-anchor", &nextPage, nil), nil
		case 2:
			requireListPage(t, request, nextPage)
			return resourceAnchorListPage(testResourceAnchorID, "target-anchor", nil, &requestID), nil
		default:
			t.Fatalf("unexpected list call %d", callNumber())
			return multicloudsdk.ListResourceAnchorsResponse{}, nil
		}
	}
}

func requireNilListPage(t *testing.T, request multicloudsdk.ListResourceAnchorsRequest) {
	t.Helper()
	if request.Page != nil {
		t.Fatalf("first list page token = %q, want nil", *request.Page)
	}
}

func requireListPage(t *testing.T, request multicloudsdk.ListResourceAnchorsRequest, want string) {
	t.Helper()
	if got := stringValue(request.Page); got != want {
		t.Fatalf("second list page token = %q, want %q", got, want)
	}
}

func resourceAnchorListPage(id string, name string, nextPage *string, requestID *string) multicloudsdk.ListResourceAnchorsResponse {
	return multicloudsdk.ListResourceAnchorsResponse{
		ResourceAnchorCollection: multicloudsdk.ResourceAnchorCollection{
			Items: []multicloudsdk.ResourceAnchorSummary{
				testResourceAnchorSummary(id, name, multicloudsdk.ResourceAnchorLifecycleStateActive),
			},
		},
		OpcNextPage:  nextPage,
		OpcRequestId: requestID,
	}
}

func testResourceAnchorClient(fake *fakeResourceAnchorOCI) resourceAnchorReadOnlyClient {
	return resourceAnchorReadOnlyClient{
		delegate: &fakeResourceAnchorDelegate{deleteResult: true},
		get:      fake.GetResourceAnchor,
		list:     fake.ListResourceAnchors,
	}
}

func testResourceAnchorResource(annotations map[string]string) *multicloudv1beta1.ResourceAnchor {
	return &multicloudv1beta1.ResourceAnchor{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "resourceanchor-sample",
			Namespace:   "default",
			Annotations: annotations,
		},
	}
}

func testResourceAnchor(id string, displayName string, lifecycle multicloudsdk.ResourceAnchorLifecycleStateEnum) multicloudsdk.ResourceAnchor {
	created := common.SDKTime{Time: time.Date(2026, time.May, 19, 12, 0, 0, 0, time.UTC)}
	updated := common.SDKTime{Time: time.Date(2026, time.May, 19, 13, 0, 0, 0, time.UTC)}
	return multicloudsdk.ResourceAnchor{
		Id:             common.String(id),
		DisplayName:    common.String(displayName),
		CompartmentId:  common.String(testResourceAnchorCompartment),
		TimeCreated:    &created,
		LifecycleState: lifecycle,
		FreeformTags:   map[string]string{"owner": "osok"},
		DefinedTags: map[string]map[string]interface{}{
			"Operations": {"CostCenter": "42"},
		},
		SystemTags: map[string]map[string]interface{}{
			"orcl-cloud": {"free-tier-retained": "true"},
		},
		SubscriptionId:        common.String(testResourceAnchorSubscription),
		Region:                common.String("us-ashburn-1"),
		CompartmentName:       common.String("base"),
		TimeUpdated:           &updated,
		LifecycleDetails:      common.String("ready"),
		SetupMode:             multicloudsdk.ResourceAnchorSetupModeAutoBind,
		LinkedCompartmentId:   common.String("ocid1.compartment.oc1..linked"),
		LinkedCompartmentName: common.String("linked"),
		SubscriptionType:      multicloudsdk.SubscriptionTypeOracledbatazure,
		CloudServiceProviderMetadataItem: multicloudsdk.AzureCloudServiceProviderMetadataItem{
			ResourceAnchorName:      common.String(displayName),
			Region:                  common.String("eastus"),
			CspResourceAnchorId:     common.String("azure-anchor-id"),
			CspResourceAnchorName:   common.String("azure-anchor"),
			ResourceAnchorUri:       common.String("/subscriptions/sub/resourceGroups/rg/providers/anchor"),
			CspAdditionalProperties: map[string]string{"AzureSubnetId": "subnet-1"},
			ResourceGroup:           common.String("rg"),
			Subscription:            common.String("azure-subscription"),
		},
	}
}

func testResourceAnchorSummary(id string, displayName string, lifecycle multicloudsdk.ResourceAnchorLifecycleStateEnum) multicloudsdk.ResourceAnchorSummary {
	created := common.SDKTime{Time: time.Date(2026, time.May, 19, 12, 0, 0, 0, time.UTC)}
	return multicloudsdk.ResourceAnchorSummary{
		Id:                            common.String(id),
		DisplayName:                   common.String(displayName),
		CompartmentId:                 common.String(testResourceAnchorCompartment),
		TimeCreated:                   &created,
		LifecycleState:                lifecycle,
		FreeformTags:                  map[string]string{"owner": "osok"},
		DefinedTags:                   map[string]map[string]interface{}{"Operations": {"CostCenter": "42"}},
		SubscriptionId:                common.String(testResourceAnchorSubscription),
		CompartmentName:               common.String("base"),
		PartnerCloudAccountIdentifier: common.String("partner-account"),
		CspResourceAnchorId:           common.String("csp-" + id),
		CspResourceAnchorName:         common.String(displayName),
		CspAdditionalProperties:       map[string]string{"AzureSubnetId": "subnet-1"},
	}
}

func requireCondition(t *testing.T, resource *multicloudv1beta1.ResourceAnchor, condition shared.OSOKConditionType) {
	t.Helper()
	for _, current := range resource.Status.OsokStatus.Conditions {
		if current.Type == condition {
			return
		}
	}
	t.Fatalf("condition %q not found in %#v", condition, resource.Status.OsokStatus.Conditions)
}
