/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package migration

import (
	"context"
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

func TestSyntheticMigrationCreateReadDelete(t *testing.T) {
	resource := &cloudmigrationsv1beta1.Migration{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-migration"},"spec":{"compartmentId":"ocid1.compartment.oc1..synthetic","displayName":"osok-replay-migration"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.migration.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "cloudmigrations", Resource: "Migration",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "migration_synthetic_crud.yaml"), Host: "https://cloudmigration.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220919", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{
			CreatedBody: createdBody, EmptyCollectionBody: "{\"items\":[]}", PresentCollectionBody: "{\"items\":[" + createdBody + "]}", CreateStatus: 0, DeleteStatus: 0,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := cloudmigrationssdk.MigrationClient{BaseClient: session.BaseClient()}
	manager := &MigrationServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newMigrationRuntimeHooks(manager, sdkClient)
	client := wrapMigrationGeneratedClient(hooks, defaultMigrationServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*cloudmigrationsv1beta1.Migration](buildMigrationGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudmigrationsv1beta1.Migration]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *cloudmigrationsv1beta1.Migration) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *cloudmigrationsv1beta1.Migration) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created Migration status has no OCI identity: %+v", current.Status)
			}
			return nil
		},
	})
}
