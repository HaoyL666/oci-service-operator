/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package serviceenvironment

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	servicemanagerproxysdk "github.com/oracle/oci-go-sdk/v65/servicemanagerproxy"
	servicemanagerproxyv1beta1 "github.com/oracle/oci-service-operator/api/servicemanagerproxy/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticServiceEnvironmentReadsExisting(t *testing.T) {
	resource := &servicemanagerproxyv1beta1.ServiceEnvironment{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"service-environment","namespace":"default"},"spec":{"compartmentId":"ocid1.compartment.oc1..replay","serviceEnvironmentId":"environment-1"}}`), resource); err != nil {
		t.Fatal(err)
	}
	observed := `{"id":"environment-1","compartmentId":"ocid1.compartment.oc1..replay","serviceEnvironmentId":"environment-1","status":"ACTIVE"}`
	metadata := ocireplay.Metadata{Service: "servicemanagerproxy", Resource: "ServiceEnvironment", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "serviceenvironment_synthetic_read.yaml"), Host: "https://service-manager-proxy.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210914", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observed}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close cassette: %v", err)
		}
	}()
	sdkClient := servicemanagerproxysdk.ServiceManagerProxyClient{BaseClient: session.BaseClient()}
	client := newServiceEnvironmentServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || string(resource.Status.OsokStatus.Ocid) == "" {
		t.Fatalf("read response=%+v status=%+v", response, resource.Status)
	}
}
