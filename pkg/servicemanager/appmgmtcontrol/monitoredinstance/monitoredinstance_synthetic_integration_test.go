/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package monitoredinstance

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	appmgmtcontrolsdk "github.com/oracle/oci-go-sdk/v65/appmgmtcontrol"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticMonitoredInstanceBindsExisting(t *testing.T) {
	resource := newMonitoredInstanceResource()
	observedBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "", "ACTIVE", map[string]any{"instanceId": "ocid1.instance.oc1..synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "appmgmtcontrol", Resource: "MonitoredInstance", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "monitoredinstance_synthetic_read.yaml"), Host: "https://appmgmt-control.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210330", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: `{"items":[` + observedBody + `]}`}, {StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close cassette: %v", err)
		}
	}()
	sdkClient := appmgmtcontrolsdk.AppmgmtControlClient{BaseClient: session.BaseClient()}
	client := newMonitoredInstanceServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.InstanceId == "" {
		t.Fatalf("bind response=%+v status=%+v", response, resource.Status)
	}
}
