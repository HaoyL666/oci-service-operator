/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package resourceanalyticsinstance

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	resourceanalyticssdk "github.com/oracle/oci-go-sdk/v65/resourceanalytics"
	resourceanalyticsv1beta1 "github.com/oracle/oci-service-operator/api/resourceanalytics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticResourceAnalyticsInstanceReadsTracked(t *testing.T) {
	resource := &resourceanalyticsv1beta1.ResourceAnalyticsInstance{}
	resourceID := "ocid1.resourceanalyticsinstance.oc1..synthetic"
	var inputValues map[string]any
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, inputValues); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "ACTIVE", map[string]any{"key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "resourceanalytics", Resource: "ResourceAnalyticsInstance", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "resourceanalyticsinstance_synthetic_read.yaml"), Host: "https://resource-analytics.us-ashburn-1.oci.oraclecloud.com", BasePath: "20241031", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := resourceanalyticssdk.ResourceAnalyticsInstanceClient{BaseClient: session.BaseClient()}
	client := newResourceAnalyticsInstanceServiceClientWithOCIClient(sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked ResourceAnalyticsInstance response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic ResourceAnalyticsInstance cassette: %w", err))
	}
}
