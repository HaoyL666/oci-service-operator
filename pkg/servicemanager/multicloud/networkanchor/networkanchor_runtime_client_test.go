/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package networkanchor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	multicloudsdk "github.com/oracle/oci-go-sdk/v65/multicloud"
	multicloudv1beta1 "github.com/oracle/oci-service-operator/api/multicloud/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil/errortest"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testNetworkAnchorID             = "ocid1.networkanchor.oc1..example"
	testNetworkAnchorOtherID        = "ocid1.networkanchor.oc1..other"
	testNetworkAnchorCompartmentID  = "ocid1.compartment.oc1..networkanchor"
	testNetworkAnchorSubscriptionID = "ocid1.multicloudsubscription.oc1..example"
	testNetworkAnchorDisplayName    = "orders-anchor"
)

type fakeNetworkAnchorOCIClient struct {
	getRequests  []multicloudsdk.GetNetworkAnchorRequest
	listRequests []multicloudsdk.ListNetworkAnchorsRequest

	getFunc  func(context.Context, multicloudsdk.GetNetworkAnchorRequest) (multicloudsdk.GetNetworkAnchorResponse, error)
	listFunc func(context.Context, multicloudsdk.ListNetworkAnchorsRequest) (multicloudsdk.ListNetworkAnchorsResponse, error)
}

func (f *fakeNetworkAnchorOCIClient) GetNetworkAnchor(ctx context.Context, request multicloudsdk.GetNetworkAnchorRequest) (multicloudsdk.GetNetworkAnchorResponse, error) {
	f.getRequests = append(f.getRequests, request)
	if f.getFunc != nil {
		return f.getFunc(ctx, request)
	}
	return multicloudsdk.GetNetworkAnchorResponse{}, nil
}

func (f *fakeNetworkAnchorOCIClient) ListNetworkAnchors(ctx context.Context, request multicloudsdk.ListNetworkAnchorsRequest) (multicloudsdk.ListNetworkAnchorsResponse, error) {
	f.listRequests = append(f.listRequests, request)
	if f.listFunc != nil {
		return f.listFunc(ctx, request)
	}
	return multicloudsdk.ListNetworkAnchorsResponse{}, nil
}

func TestNetworkAnchorCreateOrUpdateRejectsMissingBindCriteria(t *testing.T) {
	client := &fakeNetworkAnchorOCIClient{}
	resource := newTestNetworkAnchor(nil)

	response, err := newNetworkAnchorRuntimeClient(loggerutil.OSOKLogger{}, client, nil).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want missing bind criteria error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() response is successful, want failure")
	}
	if !strings.Contains(err.Error(), "read-only multicloud SDK surface") {
		t.Fatalf("CreateOrUpdate() error = %v, want read-only SDK context", err)
	}
	if len(client.getRequests) != 0 || len(client.listRequests) != 0 {
		t.Fatalf("OCI calls = get:%d list:%d, want none", len(client.getRequests), len(client.listRequests))
	}
	requireNetworkAnchorCondition(t, resource, shared.Failed, corev1.ConditionFalse)
}

func TestNetworkAnchorCreateOrUpdateRejectsInvalidSubscriptionServiceBeforeOCI(t *testing.T) {
	client := &fakeNetworkAnchorOCIClient{}
	resource := newTestNetworkAnchor(map[string]string{
		networkAnchorIDAnnotation:                  testNetworkAnchorID,
		networkAnchorSubscriptionIDAnnotation:      testNetworkAnchorSubscriptionID,
		networkAnchorSubscriptionServiceAnnotation: "NOT_A_SERVICE",
	})

	_, err := newNetworkAnchorRuntimeClient(loggerutil.OSOKLogger{}, client, nil).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want invalid subscription service error")
	}
	if !strings.Contains(err.Error(), "unsupported subscription service") {
		t.Fatalf("CreateOrUpdate() error = %v, want subscription service context", err)
	}
	if len(client.getRequests) != 0 || len(client.listRequests) != 0 {
		t.Fatalf("OCI calls = get:%d list:%d, want none", len(client.getRequests), len(client.listRequests))
	}
	requireNetworkAnchorCondition(t, resource, shared.Failed, corev1.ConditionFalse)
}

func TestNetworkAnchorCreateOrUpdateGetsExistingByRecordedIdentity(t *testing.T) {
	client := &fakeNetworkAnchorOCIClient{
		getFunc: func(_ context.Context, request multicloudsdk.GetNetworkAnchorRequest) (multicloudsdk.GetNetworkAnchorResponse, error) {
			assertNetworkAnchorGetRequest(t, request)
			return multicloudsdk.GetNetworkAnchorResponse{
				NetworkAnchor: makeSDKNetworkAnchor(testNetworkAnchorID, testNetworkAnchorDisplayName, multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateActive),
				OpcRequestId:  common.String("opc-get"),
			}, nil
		},
	}
	resource := newTestNetworkAnchor(map[string]string{
		networkAnchorIDAnnotation:                  testNetworkAnchorID,
		networkAnchorSubscriptionIDAnnotation:      testNetworkAnchorSubscriptionID,
		networkAnchorSubscriptionServiceAnnotation: "ORACLEDBATAZURE",
	})

	response, err := newNetworkAnchorRuntimeClient(loggerutil.OSOKLogger{}, client, nil).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v, want successful no requeue", response)
	}
	if len(client.getRequests) != 1 || len(client.listRequests) != 0 {
		t.Fatalf("OCI calls = get:%d list:%d, want get once", len(client.getRequests), len(client.listRequests))
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != testNetworkAnchorID {
		t.Fatalf("status.status.ocid = %q, want %q", got, testNetworkAnchorID)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-get" {
		t.Fatalf("status.status.opcRequestId = %q, want opc-get", got)
	}
	if got := resource.Status.OciMetadataItem.Vcn.VcnId; got != "ocid1.vcn.oc1..example" {
		t.Fatalf("status.ociMetadataItem.vcn.vcnId = %q, want projected VCN", got)
	}
	requireNetworkAnchorCondition(t, resource, shared.Active, corev1.ConditionTrue)
}

func TestNetworkAnchorCreateOrUpdateBindsExistingFromLaterListPage(t *testing.T) {
	client := newPagedNetworkAnchorListClient(t)
	resource := newTestNetworkAnchor(map[string]string{
		networkAnchorDisplayNameAnnotation:         testNetworkAnchorDisplayName,
		networkAnchorCompartmentIDAnnotation:       testNetworkAnchorCompartmentID,
		networkAnchorSubscriptionServiceAnnotation: "ORACLEDBATAZURE",
	})

	response, err := newNetworkAnchorRuntimeClient(loggerutil.OSOKLogger{}, client, nil).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v, want successful no requeue", response)
	}
	if len(client.getRequests) != 0 || len(client.listRequests) != 2 {
		t.Fatalf("OCI calls = get:%d list:%d, want two list pages", len(client.getRequests), len(client.listRequests))
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != testNetworkAnchorID {
		t.Fatalf("status.status.ocid = %q, want %q", got, testNetworkAnchorID)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-list-1" {
		t.Fatalf("status.status.opcRequestId = %q, want first page request ID", got)
	}
	requireNetworkAnchorCondition(t, resource, shared.Active, corev1.ConditionTrue)
}

func TestNetworkAnchorCreateOrUpdateRejectsDuplicateListMatches(t *testing.T) {
	client := &fakeNetworkAnchorOCIClient{
		listFunc: func(context.Context, multicloudsdk.ListNetworkAnchorsRequest) (multicloudsdk.ListNetworkAnchorsResponse, error) {
			return multicloudsdk.ListNetworkAnchorsResponse{
				NetworkAnchorCollection: multicloudsdk.NetworkAnchorCollection{
					Items: []multicloudsdk.NetworkAnchorSummary{
						makeSDKNetworkAnchorSummary(testNetworkAnchorID, testNetworkAnchorDisplayName, multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateActive),
						makeSDKNetworkAnchorSummary(testNetworkAnchorOtherID, testNetworkAnchorDisplayName, multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateActive),
					},
				},
				OpcRequestId: common.String("opc-list"),
			}, nil
		},
	}
	resource := newTestNetworkAnchor(map[string]string{
		networkAnchorDisplayNameAnnotation:         testNetworkAnchorDisplayName,
		networkAnchorCompartmentIDAnnotation:       testNetworkAnchorCompartmentID,
		networkAnchorSubscriptionServiceAnnotation: "ORACLEDBATAZURE",
	})

	_, err := newNetworkAnchorRuntimeClient(loggerutil.OSOKLogger{}, client, nil).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want duplicate match error")
	}
	if !strings.Contains(err.Error(), "multiple NetworkAnchors matched") {
		t.Fatalf("CreateOrUpdate() error = %v, want duplicate match context", err)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-list" {
		t.Fatalf("status.status.opcRequestId = %q, want opc-list", got)
	}
	requireNetworkAnchorCondition(t, resource, shared.Failed, corev1.ConditionFalse)
}

func TestNetworkAnchorCreateOrUpdateProjectsLifecyclePending(t *testing.T) {
	client := &fakeNetworkAnchorOCIClient{
		getFunc: func(context.Context, multicloudsdk.GetNetworkAnchorRequest) (multicloudsdk.GetNetworkAnchorResponse, error) {
			return multicloudsdk.GetNetworkAnchorResponse{
				NetworkAnchor: makeSDKNetworkAnchor(testNetworkAnchorID, testNetworkAnchorDisplayName, multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateCreating),
				OpcRequestId:  common.String("opc-get"),
			}, nil
		},
	}
	resource := newTestNetworkAnchor(map[string]string{
		networkAnchorIDAnnotation:                  testNetworkAnchorID,
		networkAnchorSubscriptionIDAnnotation:      testNetworkAnchorSubscriptionID,
		networkAnchorSubscriptionServiceAnnotation: "ORACLEDBATAZURE",
	})

	response, err := newNetworkAnchorRuntimeClient(loggerutil.OSOKLogger{}, client, nil).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || !response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v, want successful requeue", response)
	}
	current := resource.Status.OsokStatus.Async.Current
	if current == nil {
		t.Fatal("status.status.async.current = nil, want lifecycle pending operation")
	}
	if current.Source != shared.OSOKAsyncSourceLifecycle || current.Phase != shared.OSOKAsyncPhaseCreate || current.NormalizedClass != shared.OSOKAsyncClassPending {
		t.Fatalf("async.current = %+v, want lifecycle create pending", current)
	}
	requireNetworkAnchorCondition(t, resource, shared.Provisioning, corev1.ConditionTrue)
}

func TestNetworkAnchorCreateOrUpdateRecordsOCIErrorRequestID(t *testing.T) {
	serviceErr := errortest.NewServiceError(500, "InternalError", "service unavailable")
	serviceErr.OpcRequestID = "opc-error"
	client := &fakeNetworkAnchorOCIClient{
		getFunc: func(context.Context, multicloudsdk.GetNetworkAnchorRequest) (multicloudsdk.GetNetworkAnchorResponse, error) {
			return multicloudsdk.GetNetworkAnchorResponse{}, serviceErr
		},
	}
	resource := newTestNetworkAnchor(map[string]string{
		networkAnchorIDAnnotation:                  testNetworkAnchorID,
		networkAnchorSubscriptionIDAnnotation:      testNetworkAnchorSubscriptionID,
		networkAnchorSubscriptionServiceAnnotation: "ORACLEDBATAZURE",
	})

	response, err := newNetworkAnchorRuntimeClient(loggerutil.OSOKLogger{}, client, nil).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want service error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() response is successful, want failure")
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-error" {
		t.Fatalf("status.status.opcRequestId = %q, want opc-error", got)
	}
	requireNetworkAnchorCondition(t, resource, shared.Failed, corev1.ConditionFalse)
}

func TestNetworkAnchorDeleteReleasesFinalizerWithoutOCI(t *testing.T) {
	client := &fakeNetworkAnchorOCIClient{}
	resource := newTestNetworkAnchor(nil)
	resource.Status.Id = testNetworkAnchorID
	resource.Status.OsokStatus.Ocid = shared.OCID(testNetworkAnchorID)

	deleted, err := newNetworkAnchorRuntimeClient(loggerutil.OSOKLogger{}, client, nil).Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() deleted = false, want true")
	}
	if len(client.getRequests) != 0 || len(client.listRequests) != 0 {
		t.Fatalf("OCI calls = get:%d list:%d, want none", len(client.getRequests), len(client.listRequests))
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("status.status.deletedAt = nil, want timestamp")
	}
	if !strings.Contains(resource.Status.OsokStatus.Message, "delete is not supported") {
		t.Fatalf("status.status.message = %q, want unsupported delete context", resource.Status.OsokStatus.Message)
	}
	requireNetworkAnchorCondition(t, resource, shared.Terminating, corev1.ConditionTrue)
}

func TestNetworkAnchorCreateOrUpdateRejectsRepeatedListPageToken(t *testing.T) {
	client := &fakeNetworkAnchorOCIClient{
		listFunc: func(context.Context, multicloudsdk.ListNetworkAnchorsRequest) (multicloudsdk.ListNetworkAnchorsResponse, error) {
			return multicloudsdk.ListNetworkAnchorsResponse{
				OpcNextPage:  common.String("same-page"),
				OpcRequestId: common.String("opc-list"),
			}, nil
		},
	}
	resource := newTestNetworkAnchor(map[string]string{
		networkAnchorDisplayNameAnnotation:   testNetworkAnchorDisplayName,
		networkAnchorCompartmentIDAnnotation: testNetworkAnchorCompartmentID,
	})

	_, err := newNetworkAnchorRuntimeClient(loggerutil.OSOKLogger{}, client, nil).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want repeated page token error")
	}
	if !strings.Contains(err.Error(), "repeated page token") {
		t.Fatalf("CreateOrUpdate() error = %v, want repeated page token context", err)
	}
	if len(client.listRequests) != 2 {
		t.Fatalf("ListNetworkAnchors calls = %d, want 2", len(client.listRequests))
	}
	requireNetworkAnchorCondition(t, resource, shared.Failed, corev1.ConditionFalse)
}

func assertNetworkAnchorGetRequest(t *testing.T, request multicloudsdk.GetNetworkAnchorRequest) {
	t.Helper()
	if got := stringValue(request.NetworkAnchorId); got != testNetworkAnchorID {
		t.Fatalf("NetworkAnchorId = %q, want %q", got, testNetworkAnchorID)
	}
	if got := stringValue(request.SubscriptionId); got != testNetworkAnchorSubscriptionID {
		t.Fatalf("SubscriptionId = %q, want %q", got, testNetworkAnchorSubscriptionID)
	}
	if got := string(request.SubscriptionServiceName); got != "ORACLEDBATAZURE" {
		t.Fatalf("SubscriptionServiceName = %q, want ORACLEDBATAZURE", got)
	}
	if request.ShouldFetchVcnName == nil || !*request.ShouldFetchVcnName {
		t.Fatal("ShouldFetchVcnName = nil/false, want true")
	}
}

func newPagedNetworkAnchorListClient(t *testing.T) *fakeNetworkAnchorOCIClient {
	t.Helper()
	client := &fakeNetworkAnchorOCIClient{}
	client.listFunc = func(_ context.Context, request multicloudsdk.ListNetworkAnchorsRequest) (multicloudsdk.ListNetworkAnchorsResponse, error) {
		assertNetworkAnchorListBindRequest(t, request)
		return pagedNetworkAnchorListResponse(t, len(client.listRequests), request), nil
	}
	return client
}

func assertNetworkAnchorListBindRequest(t *testing.T, request multicloudsdk.ListNetworkAnchorsRequest) {
	t.Helper()
	if got := stringValue(request.DisplayName); got != testNetworkAnchorDisplayName {
		t.Fatalf("DisplayName = %q, want %q", got, testNetworkAnchorDisplayName)
	}
	if got := stringValue(request.CompartmentId); got != testNetworkAnchorCompartmentID {
		t.Fatalf("CompartmentId = %q, want %q", got, testNetworkAnchorCompartmentID)
	}
}

func pagedNetworkAnchorListResponse(t *testing.T, call int, request multicloudsdk.ListNetworkAnchorsRequest) multicloudsdk.ListNetworkAnchorsResponse {
	t.Helper()
	switch call {
	case 1:
		if request.Page != nil {
			t.Fatalf("first Page = %q, want nil", stringValue(request.Page))
		}
		return multicloudsdk.ListNetworkAnchorsResponse{
			NetworkAnchorCollection: multicloudsdk.NetworkAnchorCollection{
				Items: []multicloudsdk.NetworkAnchorSummary{
					makeSDKNetworkAnchorSummary(testNetworkAnchorOtherID, "other-anchor", multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateActive),
				},
			},
			OpcNextPage:  common.String("page-2"),
			OpcRequestId: common.String("opc-list-1"),
		}
	case 2:
		if got := stringValue(request.Page); got != "page-2" {
			t.Fatalf("second Page = %q, want page-2", got)
		}
		return multicloudsdk.ListNetworkAnchorsResponse{
			NetworkAnchorCollection: multicloudsdk.NetworkAnchorCollection{
				Items: []multicloudsdk.NetworkAnchorSummary{
					makeSDKNetworkAnchorSummary(testNetworkAnchorID, testNetworkAnchorDisplayName, multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateActive),
				},
			},
		}
	default:
		t.Fatalf("unexpected ListNetworkAnchors call %d", call)
		return multicloudsdk.ListNetworkAnchorsResponse{}
	}
}

func newTestNetworkAnchor(annotations map[string]string) *multicloudv1beta1.NetworkAnchor {
	return &multicloudv1beta1.NetworkAnchor{
		ObjectMeta: metav1.ObjectMeta{
			Name:        testNetworkAnchorDisplayName,
			Namespace:   "default",
			Annotations: annotations,
		},
	}
}

func makeSDKNetworkAnchor(id string, displayName string, lifecycle multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateEnum) multicloudsdk.NetworkAnchor {
	return multicloudsdk.NetworkAnchor{
		Id:                          common.String(id),
		DisplayName:                 common.String(displayName),
		CompartmentId:               common.String(testNetworkAnchorCompartmentID),
		ResourceAnchorId:            common.String("ocid1.resourceanchor.oc1..example"),
		TimeCreated:                 sdkTimeForTest(),
		NetworkAnchorLifecycleState: lifecycle,
		FreeformTags:                map[string]string{"team": "payments"},
		DefinedTags:                 map[string]map[string]interface{}{"Operations": {"CostCenter": "42"}},
		TimeUpdated:                 sdkTimeForTest(),
		LifecycleDetails:            common.String("ready"),
		SystemTags:                  map[string]map[string]interface{}{"orcl-cloud": {"free-tier-retained": true}},
		SetupMode:                   multicloudsdk.NetworkAnchorSetupModeNoAutoBind,
		ClusterPlacementGroupId:     common.String("ocid1.clusterplacementgroup.oc1..example"),
		OciMetadataItem: &multicloudsdk.OciNetworkMetadata{
			NetworkAnchorConnectionStatus: multicloudsdk.NetworkAnchorConnectionStatusConnected,
			Vcn: &multicloudsdk.OciVcn{
				VcnId:      common.String("ocid1.vcn.oc1..example"),
				VcnName:    common.String("vcn"),
				CidrBlocks: []string{"10.0.0.0/16"},
			},
			Subnets: []multicloudsdk.OciNetworkSubnet{{
				Type:     multicloudsdk.OciNetworkSubnetTypeClient,
				SubnetId: common.String("ocid1.subnet.oc1..example"),
				Label:    common.String("client"),
			}},
		},
		CloudServiceProviderMetadataItem: &multicloudsdk.CloudServiceProviderNetworkMetadataItem{
			Region:                  common.String("eastus"),
			OdbNetworkId:            common.String("odb-network"),
			CidrBlocks:              []string{"10.1.0.0/16"},
			NetworkAnchorUri:        common.String("/subscriptions/example/networkAnchors/orders"),
			CspAdditionalProperties: map[string]string{"AzureSubnetId": "subnet-a"},
		},
		SubscriptionType: multicloudsdk.SubscriptionTypeOracledbatazure,
	}
}

func makeSDKNetworkAnchorSummary(id string, displayName string, lifecycle multicloudsdk.NetworkAnchorNetworkAnchorLifecycleStateEnum) multicloudsdk.NetworkAnchorSummary {
	return multicloudsdk.NetworkAnchorSummary{
		Id:                            common.String(id),
		DisplayName:                   common.String(displayName),
		CompartmentId:                 common.String(testNetworkAnchorCompartmentID),
		ResourceAnchorId:              common.String("ocid1.resourceanchor.oc1..example"),
		NetworkAnchorConnectionStatus: multicloudsdk.NetworkAnchorConnectionStatusConnected,
		TimeCreated:                   sdkTimeForTest(),
		NetworkAnchorLifecycleState:   lifecycle,
		FreeformTags:                  map[string]string{"team": "payments"},
		DefinedTags:                   map[string]map[string]interface{}{"Operations": {"CostCenter": "42"}},
		VcnId:                         common.String("ocid1.vcn.oc1..example"),
		VcnName:                       common.String("vcn"),
		ClusterPlacementGroupId:       common.String("ocid1.clusterplacementgroup.oc1..example"),
		TimeUpdated:                   sdkTimeForTest(),
		CspAdditionalProperties:       map[string]string{"AzureSubnetId": "subnet-a"},
		CspNetworkAnchorId:            common.String("csp-anchor"),
		NetworkAnchorUri:              common.String("/subscriptions/example/networkAnchors/orders"),
		LifecycleDetails:              common.String("ready"),
		SystemTags:                    map[string]map[string]interface{}{"orcl-cloud": {"free-tier-retained": true}},
		SubscriptionType:              multicloudsdk.SubscriptionTypeOracledbatazure,
	}
}

func sdkTimeForTest() *common.SDKTime {
	value := common.SDKTime{Time: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)}
	return &value
}

func requireNetworkAnchorCondition(t *testing.T, resource *multicloudv1beta1.NetworkAnchor, wantType shared.OSOKConditionType, wantStatus corev1.ConditionStatus) {
	t.Helper()
	if len(resource.Status.OsokStatus.Conditions) == 0 {
		t.Fatalf("conditions = nil, want %s", wantType)
	}
	condition := resource.Status.OsokStatus.Conditions[len(resource.Status.OsokStatus.Conditions)-1]
	if condition.Type != wantType || condition.Status != wantStatus {
		t.Fatalf("last condition = %s/%s, want %s/%s", condition.Type, condition.Status, wantType, wantStatus)
	}
}
