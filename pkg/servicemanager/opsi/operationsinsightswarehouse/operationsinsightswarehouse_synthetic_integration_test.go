/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package operationsinsightswarehouse

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	opsisdk "github.com/oracle/oci-go-sdk/v65/opsi"
	opsiv1beta1 "github.com/oracle/oci-service-operator/api/opsi/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOperationsInsightsWarehouseCreateReadDelete(t *testing.T) {
	resource := newTestOperationsInsightsWarehouseResource()
	createdBody, err := ocireplay.SyntheticJSONBody(makeSDKOperationsInsightsWarehouse(testOperationsInsightsWarehouseID, resource.Spec, opsisdk.OperationsInsightsWarehouseLifecycleStateActive))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(makeOperationsInsightsWarehouseWorkRequest("ocid1.workrequest.oc1..syntheticcreate", opsisdk.OperationTypeCreateOpsiWarehouse, opsisdk.OperationStatusSucceeded, opsisdk.ActionTypeCreated))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(makeOperationsInsightsWarehouseWorkRequest("ocid1.workrequest.oc1..syntheticdelete", opsisdk.OperationTypeDeleteOpsiWarehouse, opsisdk.OperationStatusSucceeded, opsisdk.ActionTypeDeleted))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "opsi", Resource: "OperationsInsightsWarehouse", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "operationsinsightswarehouse_synthetic_crud.yaml"), Host: "https://operationsinsights.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200630", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := opsisdk.OperationsInsightsClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newOperationsInsightsWarehouseServiceClientWithOCIClient(log, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*opsiv1beta1.OperationsInsightsWarehouse]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *opsiv1beta1.OperationsInsightsWarehouse) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *opsiv1beta1.OperationsInsightsWarehouse) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created OperationsInsightsWarehouse status = %+v", current.Status)
		}
		return nil
	}})
}
