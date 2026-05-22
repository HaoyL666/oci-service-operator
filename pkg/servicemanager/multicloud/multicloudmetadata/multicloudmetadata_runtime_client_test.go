/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package multicloudmetadata

import (
	"context"
	"crypto/rsa"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	multicloudsdk "github.com/oracle/oci-go-sdk/v65/multicloud"
	multicloudv1beta1 "github.com/oracle/oci-service-operator/api/multicloud/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testCompartmentID  = "ocid1.compartment.oc1..multicloud"
	testSubscriptionID = "ocid1.subscription.oc1..multicloud"
)

type fakeMultiCloudMetadataOCIClient struct {
	getCalls  []multicloudsdk.GetMultiCloudMetadataRequest
	listCalls []multicloudsdk.ListMultiCloudMetadataRequest

	getFn  func(context.Context, multicloudsdk.GetMultiCloudMetadataRequest) (multicloudsdk.GetMultiCloudMetadataResponse, error)
	listFn func(context.Context, multicloudsdk.ListMultiCloudMetadataRequest) (multicloudsdk.ListMultiCloudMetadataResponse, error)
}

func (f *fakeMultiCloudMetadataOCIClient) GetMultiCloudMetadata(
	ctx context.Context,
	request multicloudsdk.GetMultiCloudMetadataRequest,
) (multicloudsdk.GetMultiCloudMetadataResponse, error) {
	f.getCalls = append(f.getCalls, request)
	if f.getFn != nil {
		return f.getFn(ctx, request)
	}
	return multicloudsdk.GetMultiCloudMetadataResponse{}, nil
}

func (f *fakeMultiCloudMetadataOCIClient) ListMultiCloudMetadata(
	ctx context.Context,
	request multicloudsdk.ListMultiCloudMetadataRequest,
) (multicloudsdk.ListMultiCloudMetadataResponse, error) {
	f.listCalls = append(f.listCalls, request)
	if f.listFn != nil {
		return f.listFn(ctx, request)
	}
	return multicloudsdk.ListMultiCloudMetadataResponse{}, nil
}

type opcRequestIDError struct {
	message      string
	opcRequestID string
}

func (e opcRequestIDError) Error() string {
	return e.message
}

func (e opcRequestIDError) GetOpcRequestID() string {
	return e.opcRequestID
}

type fakeMultiCloudMetadataConfigurationProvider struct {
	tenancyOCID string
	tenancyErr  error
}

func (f fakeMultiCloudMetadataConfigurationProvider) TenancyOCID() (string, error) {
	return f.tenancyOCID, f.tenancyErr
}

func (fakeMultiCloudMetadataConfigurationProvider) UserOCID() (string, error) {
	return "", nil
}

func (fakeMultiCloudMetadataConfigurationProvider) KeyFingerprint() (string, error) {
	return "", nil
}

func (fakeMultiCloudMetadataConfigurationProvider) Region() (string, error) {
	return "", nil
}

func (fakeMultiCloudMetadataConfigurationProvider) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{}, nil
}

func (fakeMultiCloudMetadataConfigurationProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	return nil, nil
}

func (fakeMultiCloudMetadataConfigurationProvider) KeyID() (string, error) {
	return "", nil
}

func TestMultiCloudMetadataCreateOrUpdateGetsAnnotatedSubscription(t *testing.T) {
	created := common.SDKTime{Time: time.Date(2026, time.May, 19, 12, 30, 0, 0, time.UTC)}
	fake := &fakeMultiCloudMetadataOCIClient{
		getFn: func(_ context.Context, request multicloudsdk.GetMultiCloudMetadataRequest) (multicloudsdk.GetMultiCloudMetadataResponse, error) {
			requireStringPtr(t, "GetMultiCloudMetadataRequest.CompartmentId", request.CompartmentId, testCompartmentID)
			requireStringPtr(t, "GetMultiCloudMetadataRequest.SubscriptionId", request.SubscriptionId, testSubscriptionID)
			return multicloudsdk.GetMultiCloudMetadataResponse{
				MultiCloudMetadata: multicloudsdk.MultiCloudMetadata{
					CompartmentId:  common.String(testCompartmentID),
					SubscriptionId: common.String(testSubscriptionID),
					TimeCreated:    &created,
					FreeformTags:   map[string]string{"owner": "networking"},
					DefinedTags: map[string]map[string]interface{}{
						"Operations": {"CostCenter": "42", "Ignored": nil, "RetryCount": 3},
					},
					SystemTags: map[string]map[string]interface{}{
						"orcl-cloud": {"free-tier-retained": "true"},
					},
				},
				OpcRequestId: common.String("req-get"),
			}, nil
		},
	}
	resource := makeMultiCloudMetadataResource(map[string]string{
		MultiCloudMetadataCompartmentIDAnnotation:  testCompartmentID,
		MultiCloudMetadataSubscriptionIDAnnotation: testSubscriptionID,
	})
	resource.Status.OsokStatus.Async.Current = &shared.OSOKAsyncOperation{
		Source:          shared.OSOKAsyncSourceLifecycle,
		Phase:           shared.OSOKAsyncPhaseCreate,
		NormalizedClass: shared.OSOKAsyncClassPending,
	}

	response, err := testMultiCloudMetadataClient(fake).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate().IsSuccessful = false, want true")
	}
	if len(fake.getCalls) != 1 {
		t.Fatalf("GetMultiCloudMetadata calls = %d, want 1", len(fake.getCalls))
	}
	if len(fake.listCalls) != 0 {
		t.Fatalf("ListMultiCloudMetadata calls = %d, want 0", len(fake.listCalls))
	}
	requireMultiCloudMetadataStatus(t, resource, testCompartmentID, testSubscriptionID)
	if got := resource.Status.TimeCreated; got != "2026-05-19T12:30:00Z" {
		t.Fatalf("status.timeCreated = %q, want 2026-05-19T12:30:00Z", got)
	}
	requireStringMapEntry(t, "status.freeformTags.owner", resource.Status.FreeformTags, "owner", "networking")
	requireMapValueEntry(t, "status.definedTags.Operations.CostCenter", resource.Status.DefinedTags, "Operations", "CostCenter", "42")
	requireMapValueEntry(t, "status.definedTags.Operations.RetryCount", resource.Status.DefinedTags, "Operations", "RetryCount", "3")
	requireMapValueAbsent(t, "status.definedTags.Operations.Ignored", resource.Status.DefinedTags, "Operations", "Ignored")
	requireMapValueEntry(t, "status.systemTags.orcl-cloud.free-tier-retained", resource.Status.SystemTags, "orcl-cloud", "free-tier-retained", "true")
	if got := resource.Status.OsokStatus.OpcRequestID; got != "req-get" {
		t.Fatalf("status.status.opcRequestId = %q, want req-get", got)
	}
	requireNoCurrentAsync(t, resource, "after successful read-only observation")
	requireCondition(t, resource, shared.Active)
}

func TestMultiCloudMetadataCreateOrUpdateUsesProviderTenancyForCompartment(t *testing.T) {
	fake := &fakeMultiCloudMetadataOCIClient{
		listFn: func(_ context.Context, request multicloudsdk.ListMultiCloudMetadataRequest) (multicloudsdk.ListMultiCloudMetadataResponse, error) {
			requireStringPtr(t, "ListMultiCloudMetadataRequest.CompartmentId", request.CompartmentId, testCompartmentID)
			return multicloudsdk.ListMultiCloudMetadataResponse{
				MultiCloudMetadataCollection: multicloudsdk.MultiCloudMetadataCollection{
					Items: []multicloudsdk.MultiCloudMetadataSummary{
						makeMultiCloudMetadataSummary(testCompartmentID, testSubscriptionID, nil),
					},
				},
			}, nil
		},
	}
	resource := makeMultiCloudMetadataResource(nil)
	provider := fakeMultiCloudMetadataConfigurationProvider{tenancyOCID: " " + testCompartmentID + " "}

	response, err := testMultiCloudMetadataClientWithProvider(fake, provider).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate().IsSuccessful = false, want true")
	}
	requireMultiCloudMetadataStatus(t, resource, testCompartmentID, testSubscriptionID)
}

func TestMultiCloudMetadataCreateOrUpdateBindsSingleMetadataAcrossListPages(t *testing.T) {
	created := common.SDKTime{Time: time.Date(2026, time.May, 19, 13, 0, 0, 0, time.UTC)}
	fake := &fakeMultiCloudMetadataOCIClient{
		listFn: func(_ context.Context, request multicloudsdk.ListMultiCloudMetadataRequest) (multicloudsdk.ListMultiCloudMetadataResponse, error) {
			requireStringPtr(t, "ListMultiCloudMetadataRequest.CompartmentId", request.CompartmentId, testCompartmentID)
			switch stringValue(request.Page) {
			case "":
				return multicloudsdk.ListMultiCloudMetadataResponse{
					OpcNextPage:  common.String("page-2"),
					OpcRequestId: common.String("req-list-1"),
				}, nil
			case "page-2":
				return multicloudsdk.ListMultiCloudMetadataResponse{
					MultiCloudMetadataCollection: multicloudsdk.MultiCloudMetadataCollection{
						Items: []multicloudsdk.MultiCloudMetadataSummary{
							makeMultiCloudMetadataSummary(testCompartmentID, testSubscriptionID, &created),
						},
					},
					OpcRequestId: common.String("req-list-2"),
				}, nil
			default:
				t.Fatalf("unexpected ListMultiCloudMetadataRequest.Page = %q", stringValue(request.Page))
				return multicloudsdk.ListMultiCloudMetadataResponse{}, nil
			}
		},
	}
	resource := makeMultiCloudMetadataResource(map[string]string{
		MultiCloudMetadataCompartmentIDAnnotation: testCompartmentID,
	})

	response, err := testMultiCloudMetadataClient(fake).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate().IsSuccessful = false, want true")
	}
	if len(fake.listCalls) != 2 {
		t.Fatalf("ListMultiCloudMetadata calls = %d, want 2", len(fake.listCalls))
	}
	requireMultiCloudMetadataStatus(t, resource, testCompartmentID, testSubscriptionID)
	if got := resource.Status.OsokStatus.OpcRequestID; got != "req-list-2" {
		t.Fatalf("status.status.opcRequestId = %q, want req-list-2", got)
	}
}

func TestMultiCloudMetadataCreateOrUpdateRejectsAmbiguousListMatches(t *testing.T) {
	fake := &fakeMultiCloudMetadataOCIClient{
		listFn: func(context.Context, multicloudsdk.ListMultiCloudMetadataRequest) (multicloudsdk.ListMultiCloudMetadataResponse, error) {
			return multicloudsdk.ListMultiCloudMetadataResponse{
				MultiCloudMetadataCollection: multicloudsdk.MultiCloudMetadataCollection{
					Items: []multicloudsdk.MultiCloudMetadataSummary{
						makeMultiCloudMetadataSummary(testCompartmentID, "ocid1.subscription.oc1..first", nil),
						makeMultiCloudMetadataSummary(testCompartmentID, "ocid1.subscription.oc1..second", nil),
					},
				},
			}, nil
		},
	}
	resource := makeMultiCloudMetadataResource(map[string]string{
		MultiCloudMetadataCompartmentIDAnnotation: testCompartmentID,
	})

	response, err := testMultiCloudMetadataClient(fake).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want ambiguous list error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate().IsSuccessful = true, want false")
	}
	if !strings.Contains(err.Error(), MultiCloudMetadataSubscriptionIDAnnotation) {
		t.Fatalf("CreateOrUpdate() error = %v, want subscription annotation guidance", err)
	}
	requireCondition(t, resource, shared.Failed)
}

func TestMultiCloudMetadataCreateOrUpdateUsesRecordedIdentityForNoOpGet(t *testing.T) {
	fake := &fakeMultiCloudMetadataOCIClient{
		getFn: func(_ context.Context, request multicloudsdk.GetMultiCloudMetadataRequest) (multicloudsdk.GetMultiCloudMetadataResponse, error) {
			requireStringPtr(t, "GetMultiCloudMetadataRequest.CompartmentId", request.CompartmentId, testCompartmentID)
			requireStringPtr(t, "GetMultiCloudMetadataRequest.SubscriptionId", request.SubscriptionId, testSubscriptionID)
			return multicloudsdk.GetMultiCloudMetadataResponse{
				MultiCloudMetadata: multicloudsdk.MultiCloudMetadata{
					CompartmentId:  common.String(testCompartmentID),
					SubscriptionId: common.String(testSubscriptionID),
				},
			}, nil
		},
		listFn: func(context.Context, multicloudsdk.ListMultiCloudMetadataRequest) (multicloudsdk.ListMultiCloudMetadataResponse, error) {
			t.Fatal("ListMultiCloudMetadata called during no-op get")
			return multicloudsdk.ListMultiCloudMetadataResponse{}, nil
		},
	}
	resource := makeMultiCloudMetadataResource(nil)
	resource.Status.CompartmentId = testCompartmentID
	resource.Status.SubscriptionId = testSubscriptionID
	resource.Status.OsokStatus.Ocid = shared.OCID(testSubscriptionID)

	response, err := testMultiCloudMetadataClient(fake).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate().IsSuccessful = false, want true")
	}
	if len(fake.getCalls) != 1 {
		t.Fatalf("GetMultiCloudMetadata calls = %d, want 1", len(fake.getCalls))
	}
	requireMultiCloudMetadataStatus(t, resource, testCompartmentID, testSubscriptionID)
}

func TestMultiCloudMetadataCreateOrUpdateRejectsIdentityDriftBeforeOCI(t *testing.T) {
	fake := &fakeMultiCloudMetadataOCIClient{
		getFn: func(context.Context, multicloudsdk.GetMultiCloudMetadataRequest) (multicloudsdk.GetMultiCloudMetadataResponse, error) {
			t.Fatal("GetMultiCloudMetadata called despite identity drift")
			return multicloudsdk.GetMultiCloudMetadataResponse{}, nil
		},
		listFn: func(context.Context, multicloudsdk.ListMultiCloudMetadataRequest) (multicloudsdk.ListMultiCloudMetadataResponse, error) {
			t.Fatal("ListMultiCloudMetadata called despite identity drift")
			return multicloudsdk.ListMultiCloudMetadataResponse{}, nil
		},
	}
	resource := makeMultiCloudMetadataResource(map[string]string{
		MultiCloudMetadataCompartmentIDAnnotation: "ocid1.compartment.oc1..different",
	})
	resource.Status.CompartmentId = testCompartmentID
	resource.Status.SubscriptionId = testSubscriptionID
	resource.Status.OsokStatus.Ocid = shared.OCID(testSubscriptionID)

	response, err := testMultiCloudMetadataClient(fake).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want identity drift error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate().IsSuccessful = true, want false")
	}
	if !strings.Contains(err.Error(), "immutable identity drift") {
		t.Fatalf("CreateOrUpdate() error = %v, want immutable identity drift", err)
	}
	requireCondition(t, resource, shared.Failed)
}

func TestMultiCloudMetadataCreateOrUpdateRecordsOCIErrorRequestID(t *testing.T) {
	fake := &fakeMultiCloudMetadataOCIClient{
		getFn: func(context.Context, multicloudsdk.GetMultiCloudMetadataRequest) (multicloudsdk.GetMultiCloudMetadataResponse, error) {
			return multicloudsdk.GetMultiCloudMetadataResponse{}, opcRequestIDError{
				message:      "service unavailable",
				opcRequestID: "req-error",
			}
		},
	}
	resource := makeMultiCloudMetadataResource(map[string]string{
		MultiCloudMetadataCompartmentIDAnnotation:  testCompartmentID,
		MultiCloudMetadataSubscriptionIDAnnotation: testSubscriptionID,
	})

	response, err := testMultiCloudMetadataClient(fake).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want OCI error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate().IsSuccessful = true, want false")
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "req-error" {
		t.Fatalf("status.status.opcRequestId = %q, want req-error", got)
	}
	requireCondition(t, resource, shared.Failed)
}

func TestMultiCloudMetadataCreateOrUpdateRequiresCompartmentIdentity(t *testing.T) {
	fake := &fakeMultiCloudMetadataOCIClient{
		getFn: func(context.Context, multicloudsdk.GetMultiCloudMetadataRequest) (multicloudsdk.GetMultiCloudMetadataResponse, error) {
			t.Fatal("GetMultiCloudMetadata called without compartment identity")
			return multicloudsdk.GetMultiCloudMetadataResponse{}, nil
		},
		listFn: func(context.Context, multicloudsdk.ListMultiCloudMetadataRequest) (multicloudsdk.ListMultiCloudMetadataResponse, error) {
			t.Fatal("ListMultiCloudMetadata called without compartment identity")
			return multicloudsdk.ListMultiCloudMetadataResponse{}, nil
		},
	}
	resource := makeMultiCloudMetadataResource(nil)

	response, err := testMultiCloudMetadataClient(fake).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want missing identity error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate().IsSuccessful = true, want false")
	}
	if !strings.Contains(err.Error(), MultiCloudMetadataCompartmentIDAnnotation) {
		t.Fatalf("CreateOrUpdate() error = %v, want compartment annotation guidance", err)
	}
	requireCondition(t, resource, shared.Failed)
}

func TestMultiCloudMetadataDeleteDoesNotCallOCIAndMarksDeleted(t *testing.T) {
	fake := &fakeMultiCloudMetadataOCIClient{
		getFn: func(context.Context, multicloudsdk.GetMultiCloudMetadataRequest) (multicloudsdk.GetMultiCloudMetadataResponse, error) {
			t.Fatal("GetMultiCloudMetadata called during delete")
			return multicloudsdk.GetMultiCloudMetadataResponse{}, nil
		},
		listFn: func(context.Context, multicloudsdk.ListMultiCloudMetadataRequest) (multicloudsdk.ListMultiCloudMetadataResponse, error) {
			t.Fatal("ListMultiCloudMetadata called during delete")
			return multicloudsdk.ListMultiCloudMetadataResponse{}, nil
		},
	}
	resource := makeMultiCloudMetadataResource(map[string]string{
		MultiCloudMetadataCompartmentIDAnnotation:  testCompartmentID,
		MultiCloudMetadataSubscriptionIDAnnotation: testSubscriptionID,
	})
	resource.Status.OsokStatus.Async.Current = &shared.OSOKAsyncOperation{
		Source:          shared.OSOKAsyncSourceLifecycle,
		Phase:           shared.OSOKAsyncPhaseDelete,
		NormalizedClass: shared.OSOKAsyncClassPending,
	}

	deleted, err := testMultiCloudMetadataClient(fake).Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() = false, want true")
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("status.status.deletedAt = nil, want timestamp")
	}
	if got := resource.Status.OsokStatus.Reason; got != string(shared.Terminating) {
		t.Fatalf("status.status.reason = %q, want %q", got, shared.Terminating)
	}
	requireNoCurrentAsync(t, resource, "after read-only delete")
	requireCondition(t, resource, shared.Terminating)
}

func testMultiCloudMetadataClient(fake *fakeMultiCloudMetadataOCIClient) MultiCloudMetadataServiceClient {
	return newMultiCloudMetadataServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("test")},
		fake,
		nil,
	)
}

func testMultiCloudMetadataClientWithProvider(
	fake *fakeMultiCloudMetadataOCIClient,
	provider common.ConfigurationProvider,
) MultiCloudMetadataServiceClient {
	return newMultiCloudMetadataServiceClientWithProvider(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("test")},
		fake,
		provider,
		nil,
	)
}

func makeMultiCloudMetadataResource(annotations map[string]string) *multicloudv1beta1.MultiCloudMetadata {
	return &multicloudv1beta1.MultiCloudMetadata{
		ObjectMeta: metav1ObjectMeta("multicloudmetadata-test", annotations),
	}
}

func metav1ObjectMeta(name string, annotations map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   "default",
		Annotations: annotations,
	}
}

func makeMultiCloudMetadataSummary(
	compartmentID string,
	subscriptionID string,
	created *common.SDKTime,
) multicloudsdk.MultiCloudMetadataSummary {
	return multicloudsdk.MultiCloudMetadataSummary{
		CompartmentId:  common.String(compartmentID),
		SubscriptionId: common.String(subscriptionID),
		TimeCreated:    created,
	}
}

func requireStringPtr(t *testing.T, name string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %q", name, want)
	}
	if *got != want {
		t.Fatalf("%s = %q, want %q", name, *got, want)
	}
}

func requireMultiCloudMetadataStatus(
	t *testing.T,
	resource *multicloudv1beta1.MultiCloudMetadata,
	wantCompartmentID string,
	wantSubscriptionID string,
) {
	t.Helper()
	if got := resource.Status.CompartmentId; got != wantCompartmentID {
		t.Fatalf("status.compartmentId = %q, want %q", got, wantCompartmentID)
	}
	if got := resource.Status.SubscriptionId; got != wantSubscriptionID {
		t.Fatalf("status.subscriptionId = %q, want %q", got, wantSubscriptionID)
	}
	if got := string(resource.Status.OsokStatus.Ocid); got != wantSubscriptionID {
		t.Fatalf("status.status.ocid = %q, want %q", got, wantSubscriptionID)
	}
}

func requireStringMapEntry(t *testing.T, name string, values map[string]string, key string, want string) {
	t.Helper()
	if got := values[key]; got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}

func requireMapValueEntry(
	t *testing.T,
	name string,
	values map[string]shared.MapValue,
	namespace string,
	key string,
	want string,
) {
	t.Helper()
	if got := values[namespace][key]; got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}

func requireMapValueAbsent(
	t *testing.T,
	name string,
	values map[string]shared.MapValue,
	namespace string,
	key string,
) {
	t.Helper()
	if _, ok := values[namespace][key]; ok {
		t.Fatalf("%s is present, want omitted", name)
	}
}

func requireNoCurrentAsync(t *testing.T, resource *multicloudv1beta1.MultiCloudMetadata, context string) {
	t.Helper()
	if resource.Status.OsokStatus.Async.Current != nil {
		t.Fatalf("status.status.async.current = %#v, want nil %s", resource.Status.OsokStatus.Async.Current, context)
	}
}

func requireCondition(
	t *testing.T,
	resource *multicloudv1beta1.MultiCloudMetadata,
	condition shared.OSOKConditionType,
) {
	t.Helper()
	for _, existing := range resource.Status.OsokStatus.Conditions {
		if existing.Type == condition {
			return
		}
	}
	t.Fatalf("condition %q not found in %#v", condition, resource.Status.OsokStatus.Conditions)
}

func TestMultiCloudMetadataListPaginationRejectsRepeatedPageToken(t *testing.T) {
	fake := &fakeMultiCloudMetadataOCIClient{
		listFn: func(_ context.Context, request multicloudsdk.ListMultiCloudMetadataRequest) (multicloudsdk.ListMultiCloudMetadataResponse, error) {
			if page := stringValue(request.Page); page != "" && page != "page-2" {
				return multicloudsdk.ListMultiCloudMetadataResponse{}, fmt.Errorf("unexpected page %q", page)
			}
			return multicloudsdk.ListMultiCloudMetadataResponse{OpcNextPage: common.String("page-2")}, nil
		},
	}
	resource := makeMultiCloudMetadataResource(map[string]string{
		MultiCloudMetadataCompartmentIDAnnotation: testCompartmentID,
	})

	_, err := testMultiCloudMetadataClient(fake).CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want repeated page token error")
	}
	if !strings.Contains(err.Error(), "repeated page token") {
		t.Fatalf("CreateOrUpdate() error = %v, want repeated page token", err)
	}
}
