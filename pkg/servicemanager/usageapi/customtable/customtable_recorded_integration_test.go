/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package customtable

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	usageapiv1beta1 "github.com/oracle/oci-service-operator/api/usageapi/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestRecordedCustomTableCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "usageapi", Resource: "CustomTable", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	tenancyID, savedReportID := "ocid1.tenancy.oc1..replay", "ocid1.usagesavedquery.oc1..replay"
	if mode == ocireplay.ModeRecord {
		tenancyID = requiredCustomTableEnv(t, "OCI_TENANCY_ID")
		savedReportID = requiredCustomTableEnv(t, "OCI_REPLAY_USAGE_SAVED_REPORT_ID")
	}
	sdkClient, closeSession := ocireplay.OpenUsageAPISDK(t, mode, filepath.Join("testdata", "recordings", "customtable_crud.yaml"), metadata)
	resource := &usageapiv1beta1.CustomTable{Spec: usageapiv1beta1.CustomTableSpec{
		CompartmentId: tenancyID,
		SavedReportId: savedReportID,
		SavedCustomTable: usageapiv1beta1.CustomTableSavedCustomTable{
			DisplayName:   "osok-replay-custom-table",
			RowGroupBy:    []string{"service"},
			ColumnGroupBy: []string{"region"},
			Version:       1,
		},
	}}
	manager := &CustomTableServiceManager{}
	hooks := newCustomTableRuntimeHooks(manager, sdkClient)
	client := wrapCustomTableGeneratedClient(hooks, defaultCustomTableServiceClient{ServiceClient: generatedruntime.NewServiceClient[*usageapiv1beta1.CustomTable](buildCustomTableGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*usageapiv1beta1.CustomTable]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *usageapiv1beta1.CustomTable) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *usageapiv1beta1.CustomTable) error {
			if current.Status.SavedCustomTable.DisplayName != "osok-replay-custom-table" {
				return fmt.Errorf("created CustomTable status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *usageapiv1beta1.CustomTable) {
			current.Spec.SavedCustomTable.DisplayName = "osok-replay-custom-table-updated"
		},
		ValidateUpdated: func(current *usageapiv1beta1.CustomTable) error {
			if current.Status.SavedCustomTable.DisplayName != "osok-replay-custom-table-updated" {
				return fmt.Errorf("updated CustomTable status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredCustomTableEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
