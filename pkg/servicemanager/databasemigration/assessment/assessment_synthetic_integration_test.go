/*
Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/
package assessment

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

func TestSyntheticAssessmentCreateReadDelete(t *testing.T) {
	resource := makeMySQLAssessmentResource()
	id := "ocid1.databaseassessment.oc1..synthetic"
	created, err := ocireplay.SyntheticObservedBody(resource.Spec, id, "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	createWR := mustAssessmentJSON(t, makeAssessmentWorkRequest("ocid1.workrequest.oc1..syntheticcreate", databasemigrationsdk.OperationTypesCreateAssessment, databasemigrationsdk.OperationStatusSucceeded, databasemigrationsdk.WorkRequestResourceActionTypeCreated, id))
	deleteWR := mustAssessmentJSON(t, makeAssessmentWorkRequest("ocid1.workrequest.oc1..syntheticdelete", databasemigrationsdk.OperationTypesDeleteAssessment, databasemigrationsdk.OperationStatusSucceeded, databasemigrationsdk.WorkRequestResourceActionTypeDeleted, id))
	metadata := ocireplay.Metadata{Service: "databasemigration", Resource: "Assessment", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "assessment_synthetic_crud.yaml"), Host: "https://database-migration.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230518", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20230518/assessments", CreatedBody: created, CreateWorkRequestBody: createWR, DeleteWorkRequestBody: deleteWR})})
	if err != nil {
		t.Fatal(err)
	}
	client := newAssessmentServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, databasemigrationsdk.DatabaseMigrationClient{BaseClient: session.BaseClient()})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*databasemigrationv1beta1.Assessment]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *databasemigrationv1beta1.Assessment) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *databasemigrationv1beta1.Assessment) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created Assessment status = %+v", current.Status)
		}
		return nil
	}})
}

func mustAssessmentJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
