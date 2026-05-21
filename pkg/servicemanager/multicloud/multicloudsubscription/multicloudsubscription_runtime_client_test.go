/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package multicloudsubscription

import (
	"context"
	"crypto/rsa"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	multicloudsdk "github.com/oracle/oci-go-sdk/v65/multicloud"
	multicloudv1beta1 "github.com/oracle/oci-service-operator/api/multicloud/v1beta1"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testCompartmentID       = "ocid1.compartment.oc1..multicloud"
	testTenancyID           = "ocid1.tenancy.oc1..multicloud"
	testSubscriptionID      = "https://console.example/subscription/ocid1.organizationssubscription.oc1..target"
	testOtherSubscriptionID = "https://console.example/subscription/ocid1.organizationssubscription.oc1..other"
	testClassicID           = "classic-subscription"
	testPartnerAccount      = "partner-account"
)

func TestMulticloudSubscriptionCreateOrUpdateBindsExistingFromLaterListPage(t *testing.T) {
	resource := newMulticloudSubscriptionResource()
	resource.Annotations = map[string]string{
		multicloudSubscriptionCompartmentIDAnnotation:  testCompartmentID,
		multicloudSubscriptionSubscriptionIDAnnotation: testSubscriptionID,
	}
	fake := &fakeMulticloudSubscriptionLister{
		responses: []multicloudsdk.ListMulticloudSubscriptionsResponse{
			{
				MulticloudSubscriptionCollection: multicloudsdk.MulticloudSubscriptionCollection{
					Items: []multicloudsdk.MulticloudSubscriptionSummary{
						newMulticloudSubscriptionSummary(testOtherSubscriptionID),
					},
				},
				OpcRequestId: common.String("opc-page-1"),
				OpcNextPage:  common.String("page-2"),
			},
			{
				MulticloudSubscriptionCollection: multicloudsdk.MulticloudSubscriptionCollection{
					Items: []multicloudsdk.MulticloudSubscriptionSummary{
						newMulticloudSubscriptionSummary(testSubscriptionID),
					},
				},
				OpcRequestId: common.String("opc-page-2"),
			},
		},
	}
	client := newTestMulticloudSubscriptionRuntimeClient(fake, fakeConfigurationProvider{tenancyID: testTenancyID})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = false, want true")
	}
	if len(fake.requests) != 2 {
		t.Fatalf("ListMulticloudSubscriptions calls = %d, want 2", len(fake.requests))
	}
	requireMulticloudSubscriptionListRequest(t, fake.requests[0], testCompartmentID, "", "")
	requireMulticloudSubscriptionListRequest(t, fake.requests[1], testCompartmentID, "page-2", "")
	requireMulticloudSubscriptionStatus(t, resource, testSubscriptionID, "opc-page-2")
}

func TestMulticloudSubscriptionCreateOrUpdateUsesTenancyAndNameFilter(t *testing.T) {
	resource := newMulticloudSubscriptionResource()
	fake := &fakeMulticloudSubscriptionLister{
		responses: []multicloudsdk.ListMulticloudSubscriptionsResponse{
			{
				MulticloudSubscriptionCollection: multicloudsdk.MulticloudSubscriptionCollection{
					Items: []multicloudsdk.MulticloudSubscriptionSummary{
						newMulticloudSubscriptionSummary(testSubscriptionID),
					},
				},
				OpcRequestId: common.String("opc-list"),
			},
		},
	}
	client := newTestMulticloudSubscriptionRuntimeClient(fake, fakeConfigurationProvider{tenancyID: testTenancyID})

	if _, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{}); err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("ListMulticloudSubscriptions calls = %d, want 1", len(fake.requests))
	}
	requireMulticloudSubscriptionListRequest(t, fake.requests[0], testTenancyID, "", resource.Name)
}

func TestMulticloudSubscriptionCreateOrUpdateRejectsMissingMatchBeforeCreate(t *testing.T) {
	resource := newMulticloudSubscriptionResource()
	resource.Annotations = map[string]string{
		multicloudSubscriptionCompartmentIDAnnotation:  testCompartmentID,
		multicloudSubscriptionSubscriptionIDAnnotation: testSubscriptionID,
	}
	fake := &fakeMulticloudSubscriptionLister{
		responses: []multicloudsdk.ListMulticloudSubscriptionsResponse{
			{OpcRequestId: common.String("opc-not-found")},
		},
	}
	client := newTestMulticloudSubscriptionRuntimeClient(fake, fakeConfigurationProvider{tenancyID: testTenancyID})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want missing subscription error")
	}
	if !strings.Contains(err.Error(), multicloudSubscriptionCreateUnsupportedMessage) {
		t.Fatalf("CreateOrUpdate() error = %q, want unsupported create context", err.Error())
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = true, want false")
	}
	if resource.Status.OsokStatus.OpcRequestID != "opc-not-found" {
		t.Fatalf("status.status.opcRequestId = %q, want opc-not-found", resource.Status.OsokStatus.OpcRequestID)
	}
	requireLastMulticloudSubscriptionCondition(t, resource, shared.Failed)
}

func TestMulticloudSubscriptionCreateOrUpdateRejectsAmbiguousListMatch(t *testing.T) {
	resource := newMulticloudSubscriptionResource()
	fake := &fakeMulticloudSubscriptionLister{
		responses: []multicloudsdk.ListMulticloudSubscriptionsResponse{
			{
				MulticloudSubscriptionCollection: multicloudsdk.MulticloudSubscriptionCollection{
					Items: []multicloudsdk.MulticloudSubscriptionSummary{
						newMulticloudSubscriptionSummary(testSubscriptionID),
						newMulticloudSubscriptionSummary(testOtherSubscriptionID),
					},
				},
			},
		},
	}
	client := newTestMulticloudSubscriptionRuntimeClient(fake, fakeConfigurationProvider{tenancyID: testTenancyID})

	if _, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{}); err == nil || !strings.Contains(err.Error(), "matched 2 OCI subscriptions") {
		t.Fatalf("CreateOrUpdate() error = %v, want ambiguous match error", err)
	}
	requireLastMulticloudSubscriptionCondition(t, resource, shared.Failed)
}

func TestMulticloudSubscriptionCreateOrUpdateRejectsSubscriptionAnnotationDrift(t *testing.T) {
	resource := newMulticloudSubscriptionResource()
	resource.Status.SubscriptionId = testSubscriptionID
	resource.Annotations = map[string]string{
		multicloudSubscriptionSubscriptionIDAnnotation: testOtherSubscriptionID,
	}
	fake := &fakeMulticloudSubscriptionLister{}
	client := newTestMulticloudSubscriptionRuntimeClient(fake, fakeConfigurationProvider{tenancyID: testTenancyID})

	if _, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{}); err == nil || !strings.Contains(err.Error(), "read-only selector annotation") {
		t.Fatalf("CreateOrUpdate() error = %v, want selector drift error", err)
	}
	if len(fake.requests) != 0 {
		t.Fatalf("ListMulticloudSubscriptions calls = %d, want 0 after drift rejection", len(fake.requests))
	}
	requireLastMulticloudSubscriptionCondition(t, resource, shared.Failed)
}

func TestMulticloudSubscriptionCreateOrUpdateRecordsListErrorRequestID(t *testing.T) {
	resource := newMulticloudSubscriptionResource()
	fake := &fakeMulticloudSubscriptionLister{
		responses: []multicloudsdk.ListMulticloudSubscriptionsResponse{
			{OpcRequestId: common.String("opc-error")},
		},
		errs: []error{errors.New("service unavailable")},
	}
	client := newTestMulticloudSubscriptionRuntimeClient(fake, fakeConfigurationProvider{tenancyID: testTenancyID})

	if _, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{}); err == nil || !strings.Contains(err.Error(), "service unavailable") {
		t.Fatalf("CreateOrUpdate() error = %v, want list failure", err)
	}
	if resource.Status.OsokStatus.OpcRequestID != "opc-error" {
		t.Fatalf("status.status.opcRequestId = %q, want opc-error", resource.Status.OsokStatus.OpcRequestID)
	}
	requireLastMulticloudSubscriptionCondition(t, resource, shared.Failed)
}

func TestMulticloudSubscriptionDeleteDoesNotIssueRemoteMutation(t *testing.T) {
	resource := newMulticloudSubscriptionResource()
	resource.Status.SubscriptionId = testSubscriptionID
	fake := &fakeMulticloudSubscriptionLister{}
	client := newTestMulticloudSubscriptionRuntimeClient(fake, fakeConfigurationProvider{tenancyID: testTenancyID})

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() = false, want true for unsupported remote delete")
	}
	if len(fake.requests) != 0 {
		t.Fatalf("ListMulticloudSubscriptions calls = %d, want 0 during delete", len(fake.requests))
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("status.status.deletedAt = nil, want deletion timestamp")
	}
	if resource.Status.OsokStatus.Message != multicloudSubscriptionDeleteUnsupportedMessage {
		t.Fatalf("status.status.message = %q, want unsupported delete message", resource.Status.OsokStatus.Message)
	}
	requireLastMulticloudSubscriptionCondition(t, resource, shared.Terminating)
}

func newTestMulticloudSubscriptionRuntimeClient(
	fake *fakeMulticloudSubscriptionLister,
	provider common.ConfigurationProvider,
) *multicloudSubscriptionRuntimeClient {
	return &multicloudSubscriptionRuntimeClient{
		list:     fake.ListMulticloudSubscriptions,
		provider: provider,
	}
}

func newMulticloudSubscriptionResource() *multicloudv1beta1.MulticloudSubscription {
	return &multicloudv1beta1.MulticloudSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Name: "multicloudsubscription-sample",
		},
	}
}

func newMulticloudSubscriptionSummary(subscriptionID string) multicloudsdk.MulticloudSubscriptionSummary {
	created := common.SDKTime{Time: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)}
	updated := common.SDKTime{Time: time.Date(2026, 5, 19, 13, 0, 0, 0, time.UTC)}
	return multicloudsdk.MulticloudSubscriptionSummary{
		ClassicSubscriptionId:         common.String(testClassicID),
		PartnerCloudAccountIdentifier: common.String(testPartnerAccount),
		TimeCreated:                   &created,
		SubscriptionId:                common.String(subscriptionID),
		ServiceName:                   multicloudsdk.SubscriptionTypeOracledbatazure,
		PaymentPlan:                   common.String("monthly"),
		ActiveCommitment:              common.String("100"),
		LifecycleState:                multicloudsdk.MulticloudSubscriptionSummaryLifecycleStateActive,
		CspAdditionalProperties:       map[string]string{"azureSubnetId": "subnet"},
		TimeUpdated:                   &updated,
		FreeformTags:                  map[string]string{"env": "test"},
		DefinedTags:                   map[string]map[string]any{"Operations": {"CostCenter": "42"}},
	}
}

func requireMulticloudSubscriptionStatus(
	t *testing.T,
	resource *multicloudv1beta1.MulticloudSubscription,
	subscriptionID string,
	requestID string,
) {
	t.Helper()

	if resource.Status.SubscriptionId != subscriptionID {
		t.Fatalf("status.subscriptionId = %q, want %q", resource.Status.SubscriptionId, subscriptionID)
	}
	if resource.Status.ClassicSubscriptionId != testClassicID {
		t.Fatalf("status.classicSubscriptionId = %q, want %q", resource.Status.ClassicSubscriptionId, testClassicID)
	}
	if resource.Status.PartnerCloudAccountIdentifier != testPartnerAccount {
		t.Fatalf("status.partnerCloudAccountIdentifier = %q, want %q", resource.Status.PartnerCloudAccountIdentifier, testPartnerAccount)
	}
	if resource.Status.LifecycleState != string(multicloudsdk.MulticloudSubscriptionSummaryLifecycleStateActive) {
		t.Fatalf("status.lifecycleState = %q, want ACTIVE", resource.Status.LifecycleState)
	}
	if resource.Status.OsokStatus.OpcRequestID != requestID {
		t.Fatalf("status.status.opcRequestId = %q, want %q", resource.Status.OsokStatus.OpcRequestID, requestID)
	}
	if resource.Status.OsokStatus.Ocid != "" {
		t.Fatalf("status.status.ocid = %q, want empty because subscriptionId is not an OCID", resource.Status.OsokStatus.Ocid)
	}
	if got := resource.Status.DefinedTags["Operations"]["CostCenter"]; got != "42" {
		t.Fatalf("status.definedTags[Operations][CostCenter] = %q, want 42", got)
	}
	requireLastMulticloudSubscriptionCondition(t, resource, shared.Active)
}

func requireMulticloudSubscriptionListRequest(
	t *testing.T,
	request multicloudsdk.ListMulticloudSubscriptionsRequest,
	compartmentID string,
	page string,
	displayName string,
) {
	t.Helper()

	if got := stringValue(request.CompartmentId); got != compartmentID {
		t.Fatalf("ListMulticloudSubscriptions compartmentId = %q, want %q", got, compartmentID)
	}
	if got := stringValue(request.Page); got != page {
		t.Fatalf("ListMulticloudSubscriptions page = %q, want %q", got, page)
	}
	if got := stringValue(request.DisplayName); got != displayName {
		t.Fatalf("ListMulticloudSubscriptions displayName = %q, want %q", got, displayName)
	}
	if request.Limit == nil || *request.Limit != multicloudSubscriptionListPageLimit {
		t.Fatalf("ListMulticloudSubscriptions limit = %#v, want %d", request.Limit, multicloudSubscriptionListPageLimit)
	}
}

func requireLastMulticloudSubscriptionCondition(
	t *testing.T,
	resource *multicloudv1beta1.MulticloudSubscription,
	condition shared.OSOKConditionType,
) {
	t.Helper()

	conditions := resource.Status.OsokStatus.Conditions
	if len(conditions) == 0 {
		t.Fatalf("status.status.conditions = nil, want trailing %s", condition)
	}
	if got := conditions[len(conditions)-1].Type; got != condition {
		t.Fatalf("last condition = %q, want %q", got, condition)
	}
}

type fakeMulticloudSubscriptionLister struct {
	requests  []multicloudsdk.ListMulticloudSubscriptionsRequest
	responses []multicloudsdk.ListMulticloudSubscriptionsResponse
	errs      []error
}

func (f *fakeMulticloudSubscriptionLister) ListMulticloudSubscriptions(
	_ context.Context,
	request multicloudsdk.ListMulticloudSubscriptionsRequest,
) (multicloudsdk.ListMulticloudSubscriptionsResponse, error) {
	f.requests = append(f.requests, request)
	if len(f.responses) == 0 {
		return multicloudsdk.ListMulticloudSubscriptionsResponse{}, nil
	}

	response := f.responses[0]
	f.responses = f.responses[1:]
	var err error
	if len(f.errs) > 0 {
		err = f.errs[0]
		f.errs = f.errs[1:]
	}
	return response, err
}

type fakeConfigurationProvider struct {
	tenancyID string
}

func (f fakeConfigurationProvider) TenancyOCID() (string, error) {
	return f.tenancyID, nil
}

func (fakeConfigurationProvider) UserOCID() (string, error) {
	return "", nil
}

func (fakeConfigurationProvider) KeyFingerprint() (string, error) {
	return "", nil
}

func (fakeConfigurationProvider) Region() (string, error) {
	return "us-ashburn-1", nil
}

func (fakeConfigurationProvider) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{}, nil
}

func (fakeConfigurationProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	return nil, nil
}

func (fakeConfigurationProvider) KeyID() (string, error) {
	return "", nil
}

func TestMulticloudSubscriptionMapValueTagsPreservesStringifiedValues(t *testing.T) {
	got := mapValueTags(map[string]map[string]any{"ns": {"string": "value", "number": 7}})
	want := map[string]shared.MapValue{"ns": {"string": "value", "number": "7"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mapValueTags() = %#v, want %#v", got, want)
	}
}
