/*
Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/
package connection

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	databasemigrationsdk "github.com/oracle/oci-go-sdk/v65/databasemigration"
	databasemigrationv1beta1 "github.com/oracle/oci-service-operator/api/databasemigration/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticConnectionCreateReadDelete(t *testing.T) {
	resource := makeMySQLConnectionResource()
	id := "ocid1.databaseconnection.oc1..synthetic"
	created, err := ocireplay.SyntheticObservedBody(resource.Spec, id, "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	createWR := mustConnectionJSON(t, makeConnectionWorkRequest("ocid1.workrequest.oc1..syntheticcreate", databasemigrationsdk.OperationTypesCreateConnection, databasemigrationsdk.OperationStatusSucceeded, databasemigrationsdk.WorkRequestResourceActionTypeCreated, id))
	deleteWR := mustConnectionJSON(t, makeConnectionWorkRequest("ocid1.workrequest.oc1..syntheticdelete", databasemigrationsdk.OperationTypesDeleteConnection, databasemigrationsdk.OperationStatusSucceeded, databasemigrationsdk.WorkRequestResourceActionTypeDeleted, id))
	metadata := ocireplay.Metadata{Service: "databasemigration", Resource: "Connection", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "connection_synthetic_crud.yaml"), Host: "https://database-migration.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230518", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20230518/connections", CreatedBody: created, CreateWorkRequestBody: createWR, DeleteWorkRequestBody: deleteWR})})
	if err != nil {
		t.Fatal(err)
	}
	client := newConnectionServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, databasemigrationsdk.DatabaseMigrationClient{BaseClient: session.BaseClient()})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*databasemigrationv1beta1.Connection]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *databasemigrationv1beta1.Connection) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *databasemigrationv1beta1.Connection) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created Connection status = %+v", current.Status)
		}
		return nil
	}})
}

func mustConnectionJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
