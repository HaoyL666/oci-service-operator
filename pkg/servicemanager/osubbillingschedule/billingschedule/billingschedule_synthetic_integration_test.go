/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package billingschedule

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	osubbillingschedulesdk "github.com/oracle/oci-go-sdk/v65/osubbillingschedule"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticBillingScheduleReadsExisting(t *testing.T) {
	resource := newBillingScheduleResource(map[string]string{billingScheduleCompartmentIDAnnotation: "ocid1.compartment.oc1..replay", billingScheduleSubscriptionIDAnnotation: "subscription-1", billingScheduleSubscribedServiceIDAnnotation: "service-1", billingScheduleOrderNumberAnnotation: "order-1", billingScheduleTimeInvoicingAnnotation: "2026-01-02T03:04:05Z", billingScheduleOriginRegionAnnotation: "us-ashburn-1"})
	observed := `[{"timeStart":"2026-01-01T03:04:05Z","timeEnd":"2026-12-31T03:04:05Z","timeInvoicing":"2026-01-02T03:04:05Z","invoiceStatus":"INVOICED","quantity":"4","netUnitPrice":"25.50","amount":"102.00","billingFrequency":"MONTHLY","orderNumber":"order-1","product":{"partNumber":"part-1","name":"product-1"}}]`
	metadata := ocireplay.Metadata{Service: "osubbillingschedule", Resource: "BillingSchedule", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "billingschedule_synthetic_read.yaml"), Host: "https://billing-schedule.us-ashburn-1.oci.oraclecloud.com", BasePath: "oalapp/service/onesubs/proxy/20210501", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observed}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close cassette: %v", err)
		}
	}()
	sdkClient := osubbillingschedulesdk.BillingScheduleClient{BaseClient: session.BaseClient()}
	client := newBillingScheduleServiceClientWithOCIClient(loggerutil.OSOKLogger{}, sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OrderNumber != "order-1" {
		t.Fatalf("read response=%+v status=%+v", response, resource.Status)
	}
}
