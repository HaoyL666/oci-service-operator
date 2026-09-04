/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package oracledbazureblobcontainer

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	dbmulticloudsdk "github.com/oracle/oci-go-sdk/v65/dbmulticloud"
	dbmulticloudv1beta1 "github.com/oracle/oci-service-operator/api/dbmulticloud/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOracleDbAzureBlobContainerReconcilesTracked(t *testing.T) {
	resource := &dbmulticloudv1beta1.OracleDbAzureBlobContainer{}
	resourceID := "ocid1.oracledbazureblobcontainer.oc1..synthetic"
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, nil); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(resource.Spec, resourceID, "ACTIVE", map[string]any{"key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z", "timeScheduleStart": "2026-01-02T03:04:05Z", "timeReleased": "2026-01-02T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "dbmulticloud", Resource: "OracleDbAzureBlobContainer", Operations: []ocireplay.Operation{ocireplay.OperationRead, ocireplay.OperationUpdate}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "oracledbazureblobcontainer_synthetic_reconcile.yaml"), Host: "https://dbmulticloud.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240501", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := dbmulticloudsdk.OracleDBAzureBlobContainerClient{BaseClient: session.BaseClient()}
	manager := &OracleDbAzureBlobContainerServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newOracleDbAzureBlobContainerRuntimeHooks(manager, sdkClient)
	client := wrapOracleDbAzureBlobContainerGeneratedClient(hooks, defaultOracleDbAzureBlobContainerServiceClient{ServiceClient: generatedruntime.NewServiceClient[*dbmulticloudv1beta1.OracleDbAzureBlobContainer](buildOracleDbAzureBlobContainerGeneratedRuntimeConfig(manager, hooks))})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked OracleDbAzureBlobContainer response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic OracleDbAzureBlobContainer cassette: %w", err))
	}
}
