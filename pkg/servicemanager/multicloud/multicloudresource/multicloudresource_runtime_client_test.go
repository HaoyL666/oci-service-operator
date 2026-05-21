/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package multicloudresource

import (
	"context"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/oracle/oci-go-sdk/v65/common"
	multicloudsdk "github.com/oracle/oci-go-sdk/v65/multicloud"
	multicloudv1beta1 "github.com/oracle/oci-service-operator/api/multicloud/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil/errortest"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testSubscriptionServiceName = "ORACLEDBATAZURE"
	testSubscriptionID          = "ocid1.multicloudsubscription.oc1..subscription"
	testResourceAnchorID        = "ocid1.resourceanchor.oc1..anchor"
	testCompartmentID           = "ocid1.compartment.oc1..compartment"
	testExternalLocation        = "eastus"
	testResourceID              = "ocid1.multicloudresource.oc1..resource"
	testOtherResourceID         = "ocid1.multicloudresource.oc1..other"
	testNetworkAnchorID         = "ocid1.networkanchor.oc1..network"
)

type fakeMulticloudResourceOCIClient struct {
	listMulticloudResourcesFn func(context.Context, multicloudsdk.ListMulticloudResourcesRequest) (multicloudsdk.ListMulticloudResourcesResponse, error)
}

type erroringMulticloudResourceConfigProvider struct{}

func (erroringMulticloudResourceConfigProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	return nil, errors.New("multicloudresource provider invalid")
}

func (erroringMulticloudResourceConfigProvider) KeyID() (string, error) {
	return "", errors.New("multicloudresource provider invalid")
}

func (erroringMulticloudResourceConfigProvider) TenancyOCID() (string, error) {
	return "", errors.New("multicloudresource provider invalid")
}

func (erroringMulticloudResourceConfigProvider) UserOCID() (string, error) {
	return "", errors.New("multicloudresource provider invalid")
}

func (erroringMulticloudResourceConfigProvider) KeyFingerprint() (string, error) {
	return "", errors.New("multicloudresource provider invalid")
}

func (erroringMulticloudResourceConfigProvider) Region() (string, error) {
	return "", errors.New("multicloudresource provider invalid")
}

func (erroringMulticloudResourceConfigProvider) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{}, nil
}

func (f *fakeMulticloudResourceOCIClient) ListMulticloudResources(
	ctx context.Context,
	req multicloudsdk.ListMulticloudResourcesRequest,
) (multicloudsdk.ListMulticloudResourcesResponse, error) {
	if f.listMulticloudResourcesFn != nil {
		return f.listMulticloudResourcesFn(ctx, req)
	}
	return multicloudsdk.ListMulticloudResourcesResponse{}, nil
}

func testMulticloudResourceClient(fake *fakeMulticloudResourceOCIClient) MulticloudResourceServiceClient {
	return newMulticloudResourceServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: logr.Discard()},
		fake,
	)
}

func newMulticloudResourceTestResource() *multicloudv1beta1.MulticloudResource {
	return &multicloudv1beta1.MulticloudResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicloud-resource",
			Namespace: "default",
			Annotations: map[string]string{
				multicloudResourceSubscriptionServiceNameAnnotation: testSubscriptionServiceName,
				multicloudResourceSubscriptionIDAnnotation:          testSubscriptionID,
				multicloudResourceIDAnnotation:                      testResourceID,
				multicloudResourceAnchorIDAnnotation:                testResourceAnchorID,
				multicloudResourceCompartmentIDAnnotation:           testCompartmentID,
				multicloudResourceExternalLocationAnnotation:        testExternalLocation,
			},
		},
	}
}

func TestMulticloudResourceCreateOrUpdateBindsExistingResourceAcrossPages(t *testing.T) {
	t.Parallel()

	resource := newMulticloudResourceTestResource()
	var calls []multicloudsdk.ListMulticloudResourcesRequest
	client := testMulticloudResourceClient(&fakeMulticloudResourceOCIClient{
		listMulticloudResourcesFn: func(_ context.Context, req multicloudsdk.ListMulticloudResourcesRequest) (multicloudsdk.ListMulticloudResourcesResponse, error) {
			calls = append(calls, req)
			switch len(calls) {
			case 1:
				requireMulticloudResourceListRequest(t, req, "")
				return multicloudsdk.ListMulticloudResourcesResponse{
					MulticloudResourceCollection: multicloudsdk.MulticloudResourceCollection{
						Items: []multicloudsdk.MulticloudResourceSummary{
							newSDKMulticloudResourceSummary(testOtherResourceID, "other-resource", multicloudsdk.MulticloudResourceSummaryLifecycleStateActive),
						},
					},
					OpcRequestId: common.String("opc-list-1"),
					OpcNextPage:  common.String("page-2"),
				}, nil
			case 2:
				requireMulticloudResourceListRequest(t, req, "page-2")
				return multicloudsdk.ListMulticloudResourcesResponse{
					MulticloudResourceCollection: multicloudsdk.MulticloudResourceCollection{
						Items: []multicloudsdk.MulticloudResourceSummary{
							newSDKMulticloudResourceSummary(testResourceID, "target-resource", multicloudsdk.MulticloudResourceSummaryLifecycleStateActive),
						},
					},
					OpcRequestId: common.String("opc-list-2"),
				}, nil
			default:
				t.Fatalf("unexpected list call %d", len(calls))
				return multicloudsdk.ListMulticloudResourcesResponse{}, nil
			}
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report success")
	}
	if response.ShouldRequeue {
		t.Fatal("CreateOrUpdate() should not requeue for ACTIVE list observation")
	}
	if len(calls) != 2 {
		t.Fatalf("list calls = %d, want 2", len(calls))
	}
	assertMulticloudResourceActiveStatus(t, resource, "target-resource", "opc-list-2")
}

func TestMulticloudResourceCreateOrUpdateUsesRecordedIdentity(t *testing.T) {
	t.Parallel()

	resource := newMulticloudResourceTestResource()
	delete(resource.Annotations, multicloudResourceIDAnnotation)
	resource.Status.ResourceId = testResourceID
	resource.Status.OsokStatus.Ocid = shared.OCID(testResourceID)

	calls := 0
	client := testMulticloudResourceClient(&fakeMulticloudResourceOCIClient{
		listMulticloudResourcesFn: func(_ context.Context, req multicloudsdk.ListMulticloudResourcesRequest) (multicloudsdk.ListMulticloudResourcesResponse, error) {
			calls++
			requireMulticloudResourceListRequest(t, req, "")
			return multicloudsdk.ListMulticloudResourcesResponse{
				MulticloudResourceCollection: multicloudsdk.MulticloudResourceCollection{
					Items: []multicloudsdk.MulticloudResourceSummary{
						newSDKMulticloudResourceSummary(testResourceID, "target-resource", multicloudsdk.MulticloudResourceSummaryLifecycleStateActive),
					},
				},
				OpcRequestId: common.String("opc-list-recorded"),
			}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report success")
	}
	if calls != 1 {
		t.Fatalf("list calls = %d, want 1", calls)
	}
	assertMulticloudResourceActiveStatus(t, resource, "target-resource", "opc-list-recorded")
}

func TestMulticloudResourceCreateOrUpdateRejectsMissingRequiredAnnotationsBeforeOCI(t *testing.T) {
	t.Parallel()

	resource := &multicloudv1beta1.MulticloudResource{}
	client := testMulticloudResourceClient(&fakeMulticloudResourceOCIClient{
		listMulticloudResourcesFn: func(context.Context, multicloudsdk.ListMulticloudResourcesRequest) (multicloudsdk.ListMulticloudResourcesResponse, error) {
			t.Fatal("ListMulticloudResources should not be called without required annotations")
			return multicloudsdk.ListMulticloudResourcesResponse{}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want missing annotation error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report failure")
	}
	for _, want := range []string{multicloudResourceSubscriptionServiceNameAnnotation, multicloudResourceSubscriptionIDAnnotation} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("CreateOrUpdate() error = %q, want %s", err.Error(), want)
		}
	}
	assertMulticloudResourceFailed(t, resource)
}

func TestMulticloudResourceCreateOrUpdateRejectsResourceIDAnnotationDrift(t *testing.T) {
	t.Parallel()

	resource := newMulticloudResourceTestResource()
	resource.Status.ResourceId = testResourceID
	resource.Status.OsokStatus.Ocid = shared.OCID(testResourceID)
	resource.Annotations[multicloudResourceIDAnnotation] = testOtherResourceID
	client := testMulticloudResourceClient(&fakeMulticloudResourceOCIClient{
		listMulticloudResourcesFn: func(context.Context, multicloudsdk.ListMulticloudResourcesRequest) (multicloudsdk.ListMulticloudResourcesResponse, error) {
			t.Fatal("ListMulticloudResources should not be called after resource-id drift")
			return multicloudsdk.ListMulticloudResourcesResponse{}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want resource-id drift error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report failure")
	}
	if !strings.Contains(err.Error(), "create-only") || !strings.Contains(err.Error(), multicloudResourceIDAnnotation) {
		t.Fatalf("CreateOrUpdate() error = %q, want create-only annotation drift", err.Error())
	}
	assertMulticloudResourceFailed(t, resource)
}

func TestMulticloudResourceCreateOrUpdateRejectsMultipleMatches(t *testing.T) {
	t.Parallel()

	resource := newMulticloudResourceTestResource()
	delete(resource.Annotations, multicloudResourceIDAnnotation)
	resource.Annotations[multicloudResourceDisplayNameAnnotation] = "duplicate-resource"
	client := testMulticloudResourceClient(&fakeMulticloudResourceOCIClient{
		listMulticloudResourcesFn: func(_ context.Context, _ multicloudsdk.ListMulticloudResourcesRequest) (multicloudsdk.ListMulticloudResourcesResponse, error) {
			return multicloudsdk.ListMulticloudResourcesResponse{
				MulticloudResourceCollection: multicloudsdk.MulticloudResourceCollection{
					Items: []multicloudsdk.MulticloudResourceSummary{
						newSDKMulticloudResourceSummary(testResourceID, "duplicate-resource", multicloudsdk.MulticloudResourceSummaryLifecycleStateActive),
						newSDKMulticloudResourceSummary(testOtherResourceID, "duplicate-resource", multicloudsdk.MulticloudResourceSummaryLifecycleStateActive),
					},
				},
			}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want multiple match error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report failure")
	}
	if !strings.Contains(err.Error(), "multiple matching resources") {
		t.Fatalf("CreateOrUpdate() error = %q, want multiple matching resources", err.Error())
	}
	assertMulticloudResourceFailed(t, resource)
}

func TestMulticloudResourceCreateOrUpdateRecordsOpcRequestIDFromOCIError(t *testing.T) {
	t.Parallel()

	resource := newMulticloudResourceTestResource()
	serviceErr := errortest.NewServiceError(500, "InternalError", "list failed")
	serviceErr.OpcRequestID = "opc-list-error"
	client := testMulticloudResourceClient(&fakeMulticloudResourceOCIClient{
		listMulticloudResourcesFn: func(_ context.Context, _ multicloudsdk.ListMulticloudResourcesRequest) (multicloudsdk.ListMulticloudResourcesResponse, error) {
			return multicloudsdk.ListMulticloudResourcesResponse{}, serviceErr
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want OCI error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report failure")
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-list-error" {
		t.Fatalf("status.opcRequestId = %q, want opc-list-error", got)
	}
	assertMulticloudResourceFailed(t, resource)
}

func TestMulticloudResourceCreateOrUpdatePreservesGeneratedOCIInitErrorWhenWrapped(t *testing.T) {
	t.Parallel()

	resource := newMulticloudResourceTestResource()
	manager := &MulticloudResourceServiceManager{
		Provider: erroringMulticloudResourceConfigProvider{},
		Log:      loggerutil.OSOKLogger{Logger: logr.Discard()},
	}
	client := newMulticloudResourceServiceClient(manager)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want OCI client initialization error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() should report failure")
	}
	if !strings.Contains(err.Error(), "initialize MulticloudResource OCI client") {
		t.Fatalf("CreateOrUpdate() error = %v, want MulticloudResource OCI client initialization failure", err)
	}
	if !strings.Contains(err.Error(), "multicloudresource provider invalid") {
		t.Fatalf("CreateOrUpdate() error = %v, want provider failure detail", err)
	}
	assertMulticloudResourceFailed(t, resource)
}

func TestMulticloudResourceDeleteReleasesFinalizerForListOnlySurface(t *testing.T) {
	t.Parallel()

	resource := newMulticloudResourceTestResource()
	resource.Status.ResourceId = testResourceID
	client := testMulticloudResourceClient(&fakeMulticloudResourceOCIClient{
		listMulticloudResourcesFn: func(context.Context, multicloudsdk.ListMulticloudResourcesRequest) (multicloudsdk.ListMulticloudResourcesResponse, error) {
			t.Fatal("ListMulticloudResources should not be called on delete for list-only surface")
			return multicloudsdk.ListMulticloudResourcesResponse{}, nil
		},
	})

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() deleted = false, want finalizer release")
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("status.deletedAt = nil, want delete timestamp")
	}
	if got := resource.Status.OsokStatus.Reason; got != string(shared.Terminating) {
		t.Fatalf("status.reason = %q, want %q", got, shared.Terminating)
	}
	if !strings.Contains(resource.Status.OsokStatus.Message, "list-only") {
		t.Fatalf("status.message = %q, want list-only context", resource.Status.OsokStatus.Message)
	}
}

func requireMulticloudResourceListRequest(t *testing.T, req multicloudsdk.ListMulticloudResourcesRequest, wantPage string) {
	t.Helper()

	if got := string(req.SubscriptionServiceName); got != testSubscriptionServiceName {
		t.Fatalf("subscriptionServiceName = %q, want %q", got, testSubscriptionServiceName)
	}
	requireStringPtrValue(t, "subscriptionId", req.SubscriptionId, testSubscriptionID)
	requireStringPtrValue(t, "resourceAnchorId", req.ResourceAnchorId, testResourceAnchorID)
	requireStringPtrValue(t, "compartmentId", req.CompartmentId, testCompartmentID)
	requireStringPtrValue(t, "externalLocation", req.ExternalLocation, testExternalLocation)
	if req.Limit == nil || *req.Limit != multicloudResourceListLimit {
		t.Fatalf("limit = %v, want %d", req.Limit, multicloudResourceListLimit)
	}
	if wantPage == "" {
		if req.Page != nil {
			t.Fatalf("page = %q, want nil", *req.Page)
		}
		return
	}
	requireStringPtrValue(t, "page", req.Page, wantPage)
}

func assertMulticloudResourceActiveStatus(
	t *testing.T,
	resource *multicloudv1beta1.MulticloudResource,
	wantDisplayName string,
	wantRequestID string,
) {
	t.Helper()

	assertMulticloudResourceObservedFields(t, resource, wantDisplayName)
	assertMulticloudResourceStatusFields(t, resource, wantRequestID)
}

func assertMulticloudResourceObservedFields(
	t *testing.T,
	resource *multicloudv1beta1.MulticloudResource,
	wantDisplayName string,
) {
	t.Helper()

	if got := resource.Status.ResourceId; got != testResourceID {
		t.Fatalf("status.resourceId = %q, want %q", got, testResourceID)
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != testResourceID {
		t.Fatalf("status.status.ocid = %q, want %q", got, testResourceID)
	}
	if got := resource.Status.ResourceDisplayName; got != wantDisplayName {
		t.Fatalf("status.resourceDisplayName = %q, want %q", got, wantDisplayName)
	}
	if got := resource.Status.CompartmentId; got != testCompartmentID {
		t.Fatalf("status.compartmentId = %q, want %q", got, testCompartmentID)
	}
	if got := resource.Status.NetworkAnchorId; got != testNetworkAnchorID {
		t.Fatalf("status.networkAnchorId = %q, want %q", got, testNetworkAnchorID)
	}
	if got := resource.Status.CspAdditionalProperties["azureSubnetId"]; got != "subnet-1" {
		t.Fatalf("status.cspAdditionalProperties[azureSubnetId] = %q, want subnet-1", got)
	}
	if got := resource.Status.DefinedTags["Operations"]["CostCenter"]; got != "42" {
		t.Fatalf("status.definedTags[Operations][CostCenter] = %q, want 42", got)
	}
}

func assertMulticloudResourceStatusFields(
	t *testing.T,
	resource *multicloudv1beta1.MulticloudResource,
	wantRequestID string,
) {
	t.Helper()

	if got := resource.Status.OsokStatus.OpcRequestID; got != wantRequestID {
		t.Fatalf("status.status.opcRequestId = %q, want %q", got, wantRequestID)
	}
	if got := resource.Status.OsokStatus.Reason; got != string(shared.Active) {
		t.Fatalf("status.status.reason = %q, want %q", got, shared.Active)
	}
	if len(resource.Status.OsokStatus.Conditions) == 0 {
		t.Fatal("status.status.conditions = empty, want Active condition")
	}
	last := resource.Status.OsokStatus.Conditions[len(resource.Status.OsokStatus.Conditions)-1]
	if last.Type != shared.Active {
		t.Fatalf("last condition = %q, want %q", last.Type, shared.Active)
	}
}

func assertMulticloudResourceFailed(t *testing.T, resource *multicloudv1beta1.MulticloudResource) {
	t.Helper()

	if got := resource.Status.OsokStatus.Reason; got != string(shared.Failed) {
		t.Fatalf("status.status.reason = %q, want %q", got, shared.Failed)
	}
	if len(resource.Status.OsokStatus.Conditions) == 0 {
		t.Fatal("status.status.conditions = empty, want Failed condition")
	}
	last := resource.Status.OsokStatus.Conditions[len(resource.Status.OsokStatus.Conditions)-1]
	if last.Type != shared.Failed {
		t.Fatalf("last condition = %q, want %q", last.Type, shared.Failed)
	}
}

func newSDKMulticloudResourceSummary(
	id string,
	displayName string,
	state multicloudsdk.MulticloudResourceSummaryLifecycleStateEnum,
) multicloudsdk.MulticloudResourceSummary {
	return multicloudsdk.MulticloudResourceSummary{
		ResourceId:              common.String(id),
		TimeCreated:             testSDKTime("2026-05-19T12:00:00Z"),
		ResourceDisplayName:     common.String(displayName),
		ResourceType:            common.String("VMCluster"),
		CompartmentName:         common.String("runtime-compartment"),
		CompartmentId:           common.String(testCompartmentID),
		VcnName:                 common.String("runtime-vcn"),
		VcnId:                   common.String("ocid1.vcn.oc1..vcn"),
		NetworkAnchorName:       common.String("runtime-network-anchor"),
		NetworkAnchorId:         common.String(testNetworkAnchorID),
		CspResourceId:           common.String("azure-resource-1"),
		CspAdditionalProperties: map[string]string{"azureSubnetId": "subnet-1"},
		TimeUpdated:             testSDKTime("2026-05-19T13:00:00Z"),
		LifecycleState:          state,
		FreeformTags:            map[string]string{"env": "test"},
		DefinedTags:             map[string]map[string]interface{}{"Operations": {"CostCenter": "42"}},
		SystemTags:              map[string]map[string]interface{}{"orcl-cloud": {"free-tier-retained": "true"}},
	}
}

func testSDKTime(value string) *common.SDKTime {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return &common.SDKTime{Time: parsed}
}

func requireStringPtrValue(t *testing.T, name string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %q", name, want)
	}
	if *got != want {
		t.Fatalf("%s = %q, want %q", name, *got, want)
	}
}
