/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package protecteddatabase

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	recoverysdk "github.com/oracle/oci-go-sdk/v65/recovery"
	recoveryv1beta1 "github.com/oracle/oci-service-operator/api/recovery/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticProtectedDatabaseCreateReadDelete(t *testing.T) {
	resource := makeProtectedDatabaseResource()
	resourceID := "ocid1.protecteddatabase.oc1..synthetic"
	createdBody, err := ocireplay.SyntheticJSONBody(makeSDKProtectedDatabase(resourceID, resource.Spec, recoverysdk.LifecycleStateActive))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(makeProtectedDatabaseWorkRequest("ocid1.workrequest.oc1..syntheticcreate", recoverysdk.OperationTypeCreateProtectedDatabase, recoverysdk.OperationStatusSucceeded, recoverysdk.ActionTypeCreated, resourceID))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(makeProtectedDatabaseWorkRequest("ocid1.workrequest.oc1..syntheticdelete", recoverysdk.OperationTypeDeleteProtectedDatabase, recoverysdk.OperationStatusSucceeded, recoverysdk.ActionTypeDeleted, resourceID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "recovery", Resource: "ProtectedDatabase", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "protecteddatabase_synthetic_crud.yaml"), Host: "https://recovery.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210216", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20210216/protectedDatabases", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := recoverysdk.DatabaseRecoveryClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newProtectedDatabaseServiceClientWithOCIClientAndCredentialClient(log, sdkClient, newFakeProtectedDatabaseCredentialClient())
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*recoveryv1beta1.ProtectedDatabase]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *recoveryv1beta1.ProtectedDatabase) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *recoveryv1beta1.ProtectedDatabase) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created ProtectedDatabase status = %+v", current.Status)
		}
		return nil
	}})
}
