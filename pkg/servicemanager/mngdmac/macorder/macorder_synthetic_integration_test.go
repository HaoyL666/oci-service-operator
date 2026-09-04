/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package macorder

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	mngdmacsdk "github.com/oracle/oci-go-sdk/v65/mngdmac"
	mngdmacv1beta1 "github.com/oracle/oci-service-operator/api/mngdmac/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticMacOrderCreateReadDelete(t *testing.T) {
	resource := newTestMacOrderResource()
	resourceID := "ocid1.macorder.oc1..synthetic"
	createdBody, err := ocireplay.SyntheticJSONBody(makeSDKMacOrder(resourceID, resource, mngdmacsdk.MacOrderLifecycleStateActive, mngdmacsdk.MacOrderOrderStatusSubmitted))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(makeMacOrderWorkRequest("ocid1.workrequest.oc1..syntheticcreate", mngdmacsdk.OperationTypeCreateMacOrder, mngdmacsdk.OperationStatusSucceeded, mngdmacsdk.ActionTypeCreated, resourceID))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(makeMacOrderWorkRequest("ocid1.workrequest.oc1..syntheticdelete", mngdmacsdk.OperationTypeCancelMacOrder, mngdmacsdk.OperationStatusSucceeded, mngdmacsdk.ActionTypeDeleted, resourceID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "mngdmac", Resource: "MacOrder", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "macorder_synthetic_crud.yaml"), Host: "https://mngdmac.us-ashburn-1.oci.oraclecloud.com", BasePath: "20250320", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, DeleteActionPathMarker: "/actions/cancel", PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := mngdmacsdk.MacOrderClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newMacOrderServiceClientWithOCIClient(log, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*mngdmacv1beta1.MacOrder]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *mngdmacv1beta1.MacOrder) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *mngdmacv1beta1.MacOrder) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created MacOrder status = %+v", current.Status)
		}
		return nil
	}})
}
