/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package replication

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	filestoragesdk "github.com/oracle/oci-go-sdk/v65/filestorage"
	filestoragev1beta1 "github.com/oracle/oci-service-operator/api/filestorage/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticReplicationReconcilesTracked(t *testing.T) {
	resource := &filestoragev1beta1.Replication{}
	resourceID := "ocid1.replication.oc1..synthetic"
	var inputValues map[string]any
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, inputValues); err != nil {
		t.Fatal(err)
	}
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "ACTIVE", map[string]any{"key": resourceID, "resourceId": resourceID, "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "filestorage", Resource: "Replication", Operations: []ocireplay.Operation{ocireplay.OperationRead, ocireplay.OperationUpdate}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "replication_synthetic_reconcile.yaml"), Host: "https://filestorage.us-ashburn-1.oci.oraclecloud.com", BasePath: "20171215", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}, {StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := filestoragesdk.FileStorageClient{BaseClient: session.BaseClient()}
	manager := &ReplicationServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newReplicationDefaultRuntimeHooks(sdkClient)
	hooks.Get.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Get.Fields)
	hooks.Update.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Update.Fields)
	client := wrapReplicationGeneratedClient(hooks, defaultReplicationServiceClient{ServiceClient: generatedruntime.NewServiceClient[*filestoragev1beta1.Replication](buildReplicationGeneratedRuntimeConfig(manager, hooks))})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked Replication response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic Replication cassette: %w", err))
	}
}
