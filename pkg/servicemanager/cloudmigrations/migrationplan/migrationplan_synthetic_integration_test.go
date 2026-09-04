/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package migrationplan

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	cloudmigrationssdk "github.com/oracle/oci-go-sdk/v65/cloudmigrations"
	cloudmigrationsv1beta1 "github.com/oracle/oci-service-operator/api/cloudmigrations/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticMigrationPlanCreateReadDelete(t *testing.T) {
	resource := &cloudmigrationsv1beta1.MigrationPlan{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"migration-plan","namespace":"default"},"spec":{"displayName":"osok-replay-plan","compartmentId":"ocid1.compartment.oc1..replay","migrationId":"ocid1.migration.oc1..replay"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.migrationplan.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "cloudmigrations", Resource: "MigrationPlan", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "migrationplan_synthetic_crud.yaml"), Host: "https://migration.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220919", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := cloudmigrationssdk.MigrationClient{BaseClient: session.BaseClient()}
	manager := &MigrationPlanServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newMigrationPlanRuntimeHooks(manager, sdkClient)
	client := wrapMigrationPlanGeneratedClient(hooks, defaultMigrationPlanServiceClient{ServiceClient: generatedruntime.NewServiceClient[*cloudmigrationsv1beta1.MigrationPlan](buildMigrationPlanGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudmigrationsv1beta1.MigrationPlan]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *cloudmigrationsv1beta1.MigrationPlan) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *cloudmigrationsv1beta1.MigrationPlan) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created MigrationPlan status = %+v", current.Status)
		}
		return nil
	}})
}
