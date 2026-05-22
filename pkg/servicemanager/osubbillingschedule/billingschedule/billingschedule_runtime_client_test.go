/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package billingschedule

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	osubbillingschedulesdk "github.com/oracle/oci-go-sdk/v65/osubbillingschedule"
	osubbillingschedulev1beta1 "github.com/oracle/oci-service-operator/api/osubbillingschedule/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestBillingScheduleCreateOrUpdateBindsExistingFromPaginatedList(t *testing.T) {
	resource := newBillingScheduleResource(map[string]string{
		billingScheduleCompartmentIDAnnotation:       "ocid1.compartment.oc1..example",
		billingScheduleSubscriptionIDAnnotation:      "subscription-1",
		billingScheduleSubscribedServiceIDAnnotation: "service-1",
		billingScheduleOrderNumberAnnotation:         "order-1",
		billingScheduleTimeInvoicingAnnotation:       "2026-01-02T03:04:05Z",
		billingScheduleOriginRegionAnnotation:        "us-phoenix-1",
	})
	nextPage := "page-2"
	fake := &fakeBillingScheduleOCIClient{
		t: t,
		listResponses: []osubbillingschedulesdk.ListBillingSchedulesResponse{
			{
				Items: []osubbillingschedulesdk.BillingScheduleSummary{
					newBillingScheduleSummary(t, "other-order", "2026-01-02T03:04:05Z"),
				},
				OpcNextPage:  common.String(nextPage),
				OpcRequestId: common.String("opc-list-1"),
			},
			{
				Items: []osubbillingschedulesdk.BillingScheduleSummary{
					newBillingScheduleSummary(t, "order-1", "2026-01-02T03:04:05Z"),
				},
				OpcRequestId: common.String("opc-list-2"),
			},
		},
	}
	client := newBillingScheduleServiceClientWithOCIClient(loggerutil.OSOKLogger{}, fake)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate returned error: %v", err)
	}
	if !response.IsSuccessful {
		t.Fatalf("CreateOrUpdate IsSuccessful = false")
	}
	if response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate ShouldRequeue = true")
	}
	if len(fake.listRequests) != 2 {
		t.Fatalf("ListBillingSchedules calls = %d, want 2", len(fake.listRequests))
	}
	assertFirstBillingScheduleListRequest(t, fake.listRequests[0])
	if got := stringValue(fake.listRequests[1].Page); got != nextPage {
		t.Fatalf("second request Page = %q, want %q", got, nextPage)
	}
	assertProjectedBillingScheduleStatus(t, resource)
	if !strings.HasPrefix(string(resource.Status.OsokStatus.Ocid), billingScheduleSyntheticIDPrefix+billingScheduleSyntheticIDVersion) {
		t.Fatalf("status ocid = %q, want synthetic BillingSchedule prefix", resource.Status.OsokStatus.Ocid)
	}
	assertBillingScheduleCondition(t, resource, shared.Active, corev1.ConditionTrue)
}

func TestBillingScheduleCreateOrUpdateRejectsMissingAnnotationsBeforeOCI(t *testing.T) {
	resource := newBillingScheduleResource(map[string]string{
		billingScheduleCompartmentIDAnnotation: "ocid1.compartment.oc1..example",
	})
	fake := &fakeBillingScheduleOCIClient{t: t}
	client := newBillingScheduleServiceClientWithOCIClient(loggerutil.OSOKLogger{}, fake)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatalf("CreateOrUpdate returned nil error")
	}
	if !strings.Contains(err.Error(), billingScheduleSubscriptionIDAnnotation) {
		t.Fatalf("error = %q, want missing subscription annotation", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate IsSuccessful = true")
	}
	if len(fake.listRequests) != 0 {
		t.Fatalf("ListBillingSchedules calls = %d, want 0", len(fake.listRequests))
	}
	assertBillingScheduleCondition(t, resource, shared.Failed, corev1.ConditionFalse)
}

func TestBillingScheduleCreateOrUpdateNoopsObservedSchedule(t *testing.T) {
	resource := newBillingScheduleResource(map[string]string{
		billingScheduleCompartmentIDAnnotation:  "ocid1.compartment.oc1..example",
		billingScheduleSubscriptionIDAnnotation: "subscription-1",
		billingScheduleOrderNumberAnnotation:    "order-1",
	})
	identity, err := resolveDesiredBillingScheduleIdentity(resource)
	if err != nil {
		t.Fatalf("resolve identity: %v", err)
	}
	resource.Status.OsokStatus.Ocid = identity.syntheticOCID()
	resource.Status.OrderNumber = "order-1"
	fake := &fakeBillingScheduleOCIClient{
		t: t,
		listResponses: []osubbillingschedulesdk.ListBillingSchedulesResponse{
			{
				Items:        []osubbillingschedulesdk.BillingScheduleSummary{newBillingScheduleSummary(t, "order-1", "2026-01-02T03:04:05Z")},
				OpcRequestId: common.String("opc-noop-list"),
			},
		},
	}
	client := newBillingScheduleServiceClientWithOCIClient(loggerutil.OSOKLogger{}, fake)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate returned error: %v", err)
	}
	if !response.IsSuccessful {
		t.Fatalf("CreateOrUpdate IsSuccessful = false")
	}
	if len(fake.listRequests) != 1 {
		t.Fatalf("ListBillingSchedules calls = %d, want 1", len(fake.listRequests))
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-noop-list" {
		t.Fatalf("status opcRequestId = %q", got)
	}
	assertBillingScheduleCondition(t, resource, shared.Active, corev1.ConditionTrue)
}

func TestBillingScheduleCreateOrUpdateDoesNotCreateMissingSchedule(t *testing.T) {
	resource := newBillingScheduleResource(map[string]string{
		billingScheduleCompartmentIDAnnotation:     "ocid1.compartment.oc1..example",
		billingScheduleSubscriptionIDAnnotation:    "subscription-1",
		billingScheduleOrderNumberAnnotation:       "order-1",
		billingScheduleTimeInvoicingAnnotation:     "2026-01-02T03:04:05Z",
		billingScheduleProductPartNumberAnnotation: "part-1",
	})
	fake := &fakeBillingScheduleOCIClient{
		t: t,
		listResponses: []osubbillingschedulesdk.ListBillingSchedulesResponse{
			{Items: nil, OpcRequestId: common.String("opc-empty-list")},
		},
	}
	client := newBillingScheduleServiceClientWithOCIClient(loggerutil.OSOKLogger{}, fake)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatalf("CreateOrUpdate returned nil error")
	}
	if !strings.Contains(err.Error(), "cannot create a missing billing schedule") {
		t.Fatalf("error = %q, want create unsupported message", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate IsSuccessful = true")
	}
	if len(fake.listRequests) != 1 {
		t.Fatalf("ListBillingSchedules calls = %d, want 1", len(fake.listRequests))
	}
	if got := resource.Status.OsokStatus.Message; !strings.Contains(got, "cannot create a missing billing schedule") {
		t.Fatalf("status message = %q", got)
	}
}

func TestBillingScheduleCreateOrUpdateRejectsAmbiguousMatches(t *testing.T) {
	resource := newBillingScheduleResource(map[string]string{
		billingScheduleCompartmentIDAnnotation:  "ocid1.compartment.oc1..example",
		billingScheduleSubscriptionIDAnnotation: "subscription-1",
		billingScheduleOrderNumberAnnotation:    "order-1",
	})
	fake := &fakeBillingScheduleOCIClient{
		t: t,
		listResponses: []osubbillingschedulesdk.ListBillingSchedulesResponse{
			{
				Items: []osubbillingschedulesdk.BillingScheduleSummary{
					newBillingScheduleSummary(t, "order-1", "2026-01-02T03:04:05Z"),
					newBillingScheduleSummary(t, "order-1", "2026-02-03T04:05:06Z"),
				},
			},
		},
	}
	client := newBillingScheduleServiceClientWithOCIClient(loggerutil.OSOKLogger{}, fake)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatalf("CreateOrUpdate returned nil error")
	}
	if !strings.Contains(err.Error(), "matched 2 OCI list items") {
		t.Fatalf("error = %q, want ambiguity message", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate IsSuccessful = true")
	}
}

func TestBillingScheduleCreateOrUpdateRejectsTrackedIdentityDriftBeforeOCI(t *testing.T) {
	oldResource := newBillingScheduleResource(map[string]string{
		billingScheduleCompartmentIDAnnotation:  "ocid1.compartment.oc1..example",
		billingScheduleSubscriptionIDAnnotation: "subscription-1",
		billingScheduleOrderNumberAnnotation:    "order-1",
	})
	oldIdentity, err := resolveDesiredBillingScheduleIdentity(oldResource)
	if err != nil {
		t.Fatalf("resolve old identity: %v", err)
	}
	resource := newBillingScheduleResource(map[string]string{
		billingScheduleCompartmentIDAnnotation:  "ocid1.compartment.oc1..example",
		billingScheduleSubscriptionIDAnnotation: "subscription-1",
		billingScheduleOrderNumberAnnotation:    "order-2",
	})
	resource.Status.OsokStatus.Ocid = oldIdentity.syntheticOCID()
	fake := &fakeBillingScheduleOCIClient{t: t}
	client := newBillingScheduleServiceClientWithOCIClient(loggerutil.OSOKLogger{}, fake)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatalf("CreateOrUpdate returned nil error")
	}
	if !strings.Contains(err.Error(), "identity is immutable") {
		t.Fatalf("error = %q, want immutable identity message", err)
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate IsSuccessful = true")
	}
	if len(fake.listRequests) != 0 {
		t.Fatalf("ListBillingSchedules calls = %d, want 0", len(fake.listRequests))
	}
}

func TestBillingScheduleCreateOrUpdateRecordsOCIErrorRequestID(t *testing.T) {
	resource := newBillingScheduleResource(map[string]string{
		billingScheduleCompartmentIDAnnotation:  "ocid1.compartment.oc1..example",
		billingScheduleSubscriptionIDAnnotation: "subscription-1",
		billingScheduleOrderNumberAnnotation:    "order-1",
	})
	fake := &fakeBillingScheduleOCIClient{
		t: t,
		listErrs: []error{fakeBillingScheduleServiceError{
			statusCode:   500,
			code:         "InternalError",
			message:      "list failed",
			opcRequestID: "opc-list-error",
		}},
	}
	client := newBillingScheduleServiceClientWithOCIClient(loggerutil.OSOKLogger{}, fake)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatalf("CreateOrUpdate returned nil error")
	}
	if response.IsSuccessful {
		t.Fatalf("CreateOrUpdate IsSuccessful = true")
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-list-error" {
		t.Fatalf("status opcRequestId = %q", got)
	}
	assertBillingScheduleCondition(t, resource, shared.Failed, corev1.ConditionFalse)
}

func TestBillingScheduleDeleteMarksReadOnlyResourceDeletedWithoutOCI(t *testing.T) {
	resource := newBillingScheduleResource(map[string]string{
		billingScheduleCompartmentIDAnnotation:  "ocid1.compartment.oc1..example",
		billingScheduleSubscriptionIDAnnotation: "subscription-1",
		billingScheduleOrderNumberAnnotation:    "order-1",
	})
	fake := &fakeBillingScheduleOCIClient{t: t}
	client := newBillingScheduleServiceClientWithOCIClient(loggerutil.OSOKLogger{}, fake)

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if !deleted {
		t.Fatalf("Delete deleted = false")
	}
	if len(fake.listRequests) != 0 {
		t.Fatalf("ListBillingSchedules calls = %d, want 0", len(fake.listRequests))
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatalf("status deletedAt was not set")
	}
	if got := resource.Status.OsokStatus.Message; !strings.Contains(got, "read-only in the OCI SDK") {
		t.Fatalf("status message = %q", got)
	}
	assertBillingScheduleCondition(t, resource, shared.Terminating, corev1.ConditionTrue)
}

func assertFirstBillingScheduleListRequest(
	t testing.TB,
	request osubbillingschedulesdk.ListBillingSchedulesRequest,
) {
	t.Helper()
	if got := stringValue(request.CompartmentId); got != "ocid1.compartment.oc1..example" {
		t.Fatalf("CompartmentId = %q", got)
	}
	if got := stringValue(request.SubscriptionId); got != "subscription-1" {
		t.Fatalf("SubscriptionId = %q", got)
	}
	if got := stringValue(request.SubscribedServiceId); got != "service-1" {
		t.Fatalf("SubscribedServiceId = %q", got)
	}
	if got := stringValue(request.XOneOriginRegion); got != "us-phoenix-1" {
		t.Fatalf("XOneOriginRegion = %q", got)
	}
	if got := intValue(request.Limit); got != billingScheduleDefaultPageLimit {
		t.Fatalf("Limit = %d, want %d", got, billingScheduleDefaultPageLimit)
	}
}

func assertProjectedBillingScheduleStatus(
	t testing.TB,
	resource *osubbillingschedulev1beta1.BillingSchedule,
) {
	t.Helper()
	if got := resource.Status.OrderNumber; got != "order-1" {
		t.Fatalf("status orderNumber = %q", got)
	}
	if got := resource.Status.TimeInvoicing; got != "2026-01-02T03:04:05Z" {
		t.Fatalf("status timeInvoicing = %q", got)
	}
	if got := resource.Status.InvoiceStatus; got != string(osubbillingschedulesdk.BillingScheduleSummaryInvoiceStatusInvoiced) {
		t.Fatalf("status invoiceStatus = %q", got)
	}
	if got := resource.Status.Quantity; got != "4" {
		t.Fatalf("status quantity = %q", got)
	}
	if got := resource.Status.Product.PartNumber; got != "part-1" {
		t.Fatalf("status product.partNumber = %q", got)
	}
	if got := resource.Status.OsokStatus.OpcRequestID; got != "opc-list-2" {
		t.Fatalf("status opcRequestId = %q", got)
	}
}

func assertBillingScheduleCondition(
	t testing.TB,
	resource *osubbillingschedulev1beta1.BillingSchedule,
	conditionType shared.OSOKConditionType,
	wantStatus corev1.ConditionStatus,
) {
	t.Helper()
	condition := findBillingScheduleCondition(resource, conditionType)
	if condition == nil {
		t.Fatalf("%s condition was not recorded", conditionType)
		return
	}
	if condition.Status != wantStatus {
		t.Fatalf("%s condition status = %s, want %s", conditionType, condition.Status, wantStatus)
	}
}

type fakeBillingScheduleOCIClient struct {
	t             testing.TB
	listResponses []osubbillingschedulesdk.ListBillingSchedulesResponse
	listErrs      []error
	listRequests  []osubbillingschedulesdk.ListBillingSchedulesRequest
}

type fakeBillingScheduleServiceError struct {
	statusCode   int
	code         string
	message      string
	opcRequestID string
}

func (e fakeBillingScheduleServiceError) Error() string {
	return e.message
}

func (e fakeBillingScheduleServiceError) GetHTTPStatusCode() int {
	return e.statusCode
}

func (e fakeBillingScheduleServiceError) GetMessage() string {
	return e.message
}

func (e fakeBillingScheduleServiceError) GetCode() string {
	return e.code
}

func (e fakeBillingScheduleServiceError) GetOpcRequestID() string {
	return e.opcRequestID
}

var _ common.ServiceError = fakeBillingScheduleServiceError{}

func (f *fakeBillingScheduleOCIClient) ListBillingSchedules(
	_ context.Context,
	request osubbillingschedulesdk.ListBillingSchedulesRequest,
) (osubbillingschedulesdk.ListBillingSchedulesResponse, error) {
	f.listRequests = append(f.listRequests, request)
	var response osubbillingschedulesdk.ListBillingSchedulesResponse
	var err error
	if len(f.listResponses) > 0 {
		response = f.listResponses[0]
		f.listResponses = f.listResponses[1:]
	}
	if len(f.listErrs) > 0 {
		err = f.listErrs[0]
		f.listErrs = f.listErrs[1:]
	}
	if f.t != nil && err == nil && response.Items == nil && response.OpcNextPage == nil && response.OpcRequestId == nil {
		f.t.Fatalf("unexpected ListBillingSchedules request: %#v", request)
	}
	return response, err
}

func newBillingScheduleResource(annotations map[string]string) *osubbillingschedulev1beta1.BillingSchedule {
	return &osubbillingschedulev1beta1.BillingSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "billing-schedule",
			Namespace:   "default",
			Annotations: annotations,
		},
	}
}

func newBillingScheduleSummary(
	t testing.TB,
	orderNumber string,
	timeInvoicing string,
) osubbillingschedulesdk.BillingScheduleSummary {
	t.Helper()
	return osubbillingschedulesdk.BillingScheduleSummary{
		TimeStart:               sdkTime(t, "2026-01-01T03:04:05Z"),
		TimeEnd:                 sdkTime(t, "2026-12-31T03:04:05Z"),
		TimeInvoicing:           sdkTime(t, timeInvoicing),
		InvoiceStatus:           osubbillingschedulesdk.BillingScheduleSummaryInvoiceStatusInvoiced,
		Quantity:                common.String("4"),
		NetUnitPrice:            common.String("25.50"),
		Amount:                  common.String("102.00"),
		BillingFrequency:        common.String("MONTHLY"),
		ArInvoiceNumber:         common.String("invoice-1"),
		ArCustomerTransactionId: common.String("transaction-1"),
		OrderNumber:             common.String(orderNumber),
		Product:                 &osubbillingschedulesdk.Product{PartNumber: common.String("part-1"), Name: common.String("product-1")},
	}
}

func sdkTime(t testing.TB, value string) *common.SDKTime {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return &common.SDKTime{Time: parsed}
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func findBillingScheduleCondition(
	resource *osubbillingschedulev1beta1.BillingSchedule,
	conditionType shared.OSOKConditionType,
) *shared.OSOKCondition {
	for i := range resource.Status.OsokStatus.Conditions {
		if resource.Status.OsokStatus.Conditions[i].Type == conditionType {
			return &resource.Status.OsokStatus.Conditions[i]
		}
	}
	return nil
}
