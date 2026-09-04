/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package operationsinsightswarehouseuser

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
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOperationsInsightsWarehouseUserReadsTracked(t *testing.T) {
	resource := &opsiv1beta1.OperationsInsightsWarehouseUser{}
	resourceID := "ocid1.operationsinsightswarehouseuser.oc1..synthetic"
	var inputValues map[string]any
	if err := ocireplay.SeedSyntheticTrackedResource(resource, resourceID, inputValues); err != nil {
		t.Fatal(err)
	}
	resource.Spec.OperationsInsightsWarehouseId = "ocid1.operationsinsightswarehouse.oc1..synthetic"
	resource.Spec.CompartmentId = "ocid1.compartment.oc1..synthetic"
	resource.Spec.Name = "SYNTHETIC_USER"
	resource.Spec.ConnectionPassword = "synthetic-password-not-recorded"
	observedBody, err := ocireplay.SyntheticObservedBody(map[string]any{}, resourceID, "ACTIVE", map[string]any{"compartmentId": "ocid1.compartment.oc1..synthetic", "isAwrDataAccess": false, "isEmDataAccess": false, "isOpsiDataAccess": false, "key": resourceID, "name": "SYNTHETIC_USER", "operationsInsightsWarehouseId": "ocid1.operationsinsightswarehouse.oc1..synthetic", "resourceId": resourceID, "status": "ACTIVE", "timeCreated": "2026-01-02T03:04:05Z", "timeUpdated": "2026-01-03T03:04:05Z"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "opsi", Resource: "OperationsInsightsWarehouseUser", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "operationsinsightswarehouseuser_synthetic_read.yaml"), Host: "https://operationsinsights.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200630", Metadata: metadata, Bindings: map[string]string{"compartment": "ocid1.compartment.oc1..synthetic", "warehouse": "ocid1.operationsinsightswarehouse.oc1..synthetic"}, Responses: []ocireplay.SyntheticResponse{{StatusCode: http.StatusOK, Body: observedBody}}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := opsisdk.OperationsInsightsClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	manager := &OperationsInsightsWarehouseUserServiceManager{Log: log}
	hooks := newOperationsInsightsWarehouseUserRuntimeHooksWithOCIClient(sdkClient)
	applyOperationsInsightsWarehouseUserRuntimeHooks(&hooks, sdkClient, nil, log)
	hooks.Get.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Get.Fields)
	hooks.Update.Fields = ocireplay.PreferTrackedIdentityForUnrepresentedPaths(resource, hooks.Update.Fields)
	client := wrapOperationsInsightsWarehouseUserGeneratedClient(hooks, defaultOperationsInsightsWarehouseUserServiceClient{ServiceClient: generatedruntime.NewServiceClient[*opsiv1beta1.OperationsInsightsWarehouseUser](buildOperationsInsightsWarehouseUserGeneratedRuntimeConfig(manager, hooks))})
	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("tracked OperationsInsightsWarehouseUser response=%+v status=%+v", response, resource.Status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(fmt.Errorf("close synthetic OperationsInsightsWarehouseUser cassette: %w", err))
	}
}
