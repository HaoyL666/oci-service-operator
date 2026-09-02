/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dbsystem

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	psqlsdk "github.com/oracle/oci-go-sdk/v65/psql"
	psqlv1beta1 "github.com/oracle/oci-service-operator/api/psql/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// PostgreSQL DbSystem creation allocates paid database compute and storage.
func TestSyntheticDbSystemCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "psql", Resource: "DbSystem", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: filepath.Join("testdata", "recordings", "dbsystem_synthetic_crud.yaml"), Host: "https://postgresql.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220915", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := psqlsdk.PostgresqlClient{BaseClient: session.BaseClient()}
	client := manualDbSystemServiceClient{sdk: sdkClient, log: discardDbSystemLogger()}
	resource := testDbSystemResource()
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*psqlv1beta1.DbSystem]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *psqlv1beta1.DbSystem) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *psqlv1beta1.DbSystem) error {
			if current.Status.DisplayName != "sample-db" || current.Status.LifecycleState != "ACTIVE" {
				return fmt.Errorf("created DbSystem status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *psqlv1beta1.DbSystem) { current.Spec.DisplayName = "sample-db-updated" },
		ValidateUpdated: func(current *psqlv1beta1.DbSystem) error {
			if current.Status.DisplayName != "sample-db-updated" {
				return fmt.Errorf("updated DbSystem status = %+v", current.Status)
			}
			return nil
		},
	})
}
