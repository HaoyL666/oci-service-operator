/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package macdevice

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	mngdmacsdk "github.com/oracle/oci-go-sdk/v65/mngdmac"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticMacDeviceReadsExisting(t *testing.T) {
	resource := newTestMacDeviceResource()
	observed, err := ocireplay.SyntheticObservedBody(resource.Spec, testMacDeviceID, "ACTIVE", map[string]any{"compartmentId": testMacCompartment, "serialNumber": testMacSerial, "ipAddress": testMacIP, "shape": "M4_PRO_MAC_MINI_64GB_2TB"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "mngdmac", Resource: "MacDevice", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "macdevice_synthetic_read.yaml"), Host: "https://mngdmac.us-ashburn-1.oci.oraclecloud.com", BasePath: "20250320", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observed}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close cassette: %v", err)
		}
	}()
	baseClient := session.BaseClient()
	deviceClient := mngdmacsdk.MacDeviceClient{BaseClient: baseClient}
	workRequestClient := mngdmacsdk.MacOrderClient{BaseClient: baseClient}
	client := newMacDeviceServiceClientWithClients(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, deviceClient, workRequestClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.Id != testMacDeviceID {
		t.Fatalf("read response=%+v status=%+v", response, resource.Status)
	}
}
