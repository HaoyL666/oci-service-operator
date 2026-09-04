/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package instanceagentplugin

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	computeinstanceagentsdk "github.com/oracle/oci-go-sdk/v65/computeinstanceagent"
	computeinstanceagentv1beta1 "github.com/oracle/oci-service-operator/api/computeinstanceagent/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticInstanceAgentPluginReadsExisting(t *testing.T) {
	resource := &computeinstanceagentv1beta1.InstanceAgentPlugin{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"plugin","namespace":"default"},"spec":{"instanceagentId":"ocid1.instance.oc1..replay","compartmentId":"ocid1.compartment.oc1..replay","pluginName":"Vulnerability Scanning"}}`), resource); err != nil {
		t.Fatal(err)
	}
	observed := `{"name":"Vulnerability Scanning","status":"RUNNING","isEnabled":true}`
	metadata := ocireplay.Metadata{Service: "computeinstanceagent", Resource: "InstanceAgentPlugin", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "instanceagentplugin_synthetic_read.yaml"), Host: "https://iaas.us-ashburn-1.oci.oraclecloud.com", BasePath: "20180530", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observed}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close cassette: %v", err)
		}
	}()
	sdkClient := computeinstanceagentsdk.PluginClient{BaseClient: session.BaseClient()}
	client := newInstanceAgentPluginServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.Name != resource.Spec.PluginName {
		t.Fatalf("read response=%+v status=%+v", response, resource.Status)
	}
}
