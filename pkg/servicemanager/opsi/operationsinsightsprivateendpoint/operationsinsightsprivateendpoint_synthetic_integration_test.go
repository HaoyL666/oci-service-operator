/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package operationsinsightsprivateendpoint

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	opsisdk "github.com/oracle/oci-go-sdk/v65/opsi"
	opsiv1beta1 "github.com/oracle/oci-service-operator/api/opsi/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOperationsInsightsPrivateEndpointReadsTracked(t *testing.T) {
	resource := &opsiv1beta1.OperationsInsightsPrivateEndpoint{}
	resourceID := "ocid1.operationsinsightsprivateendpoint.oc1..synthetic"
	var inputValues map[string]any
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, inputValues); err != nil {
		t.Fatal(err)
	}
	resource.Spec.DisplayName = "synthetic-private-endpoint"
	resource.Spec.CompartmentId = "ocid1.compartment.oc1..synthetic"
	resource.Spec.VcnId = "ocid1.vcn.oc1..synthetic"
	resource.Spec.SubnetId = "ocid1.subnet.oc1..synthetic"
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "ACTIVE", map[string]any{"compartmentId": "ocid1.compartment.oc1..synthetic", "displayName": "synthetic-private-endpoint", "key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "subnetId": "ocid1.subnet.oc1..synthetic", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z", "vcnId": "ocid1.vcn.oc1..synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "opsi", Resource: "OperationsInsightsPrivateEndpoint", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "operationsinsightsprivateendpoint_synthetic_read.yaml"), Host: "https://operationsinsights.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200630", Metadata: metadata, Bindings: map[string]string{"compartment": "ocid1.compartment.oc1..synthetic", "subnet": "ocid1.subnet.oc1..synthetic", "vcn": "ocid1.vcn.oc1..synthetic"}, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := opsisdk.OperationsInsightsClient{BaseClient: session.BaseClient()}
	client := newOperationsInsightsPrivateEndpointServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked OperationsInsightsPrivateEndpoint response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic OperationsInsightsPrivateEndpoint cassette: %w", err))
	}
}
