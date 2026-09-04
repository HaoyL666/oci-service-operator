/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package managedinstance

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	wlmssdk "github.com/oracle/oci-go-sdk/v65/wlms"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticManagedInstanceBindsExisting(t *testing.T) {
	resource := newManagedInstanceResource()
	observed, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.wlmsmanagedinstance.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "wlms", Resource: "ManagedInstance", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "managedinstance_synthetic_read.yaml"), Host: "https://wlms.us-ashburn-1.oci.oraclecloud.com", BasePath: "20241101", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: `{"items":[` + observed + `]}`}, {StatusCode: http.StatusOK, Body: observed}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close cassette: %v", err)
		}
	}()
	sdkClient := wlmssdk.WeblogicManagementServiceClient{BaseClient: session.BaseClient()}
	client := newManagedInstanceServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.Id == "" {
		t.Fatalf("bind response=%+v status=%+v", response, resource.Status)
	}
}
