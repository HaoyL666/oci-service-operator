/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package usagecarbonemissionsquery

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

func TestRecordedUsageCarbonEmissionsQueryCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "usageapi", Resource: "UsageCarbonEmissionsQuery", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	tenancyID := "ocid1.tenancy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		tenancyID = requiredUsageCarbonEmissionsQueryEnv(t, "OCI_TENANCY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenUsageAPISDK(t, mode, filepath.Join("testdata", "recordings", "usagecarbonemissionsquery_crud.yaml"), metadata)
	resource := &usageapiv1beta1.UsageCarbonEmissionsQuery{Spec: usageapiv1beta1.UsageCarbonEmissionsQuerySpec{
		CompartmentId: tenancyID,
		QueryDefinition: usageapiv1beta1.UsageCarbonEmissionsQueryQueryDefinition{
			DisplayName: "osok-replay-carbon-query",
			Version:     1,
			ReportQuery: usageapiv1beta1.UsageCarbonEmissionsQueryQueryDefinitionReportQuery{
				TenantId:                  tenancyID,
				TimeUsageStarted:          "2026-08-01T00:00:00Z",
				TimeUsageEnded:            "2026-09-01T00:00:00Z",
				EmissionCalculationMethod: "SPEND_BASED",
				EmissionType:              "LOCATION_BASED",
				Granularity:               "MONTHLY",
				GroupBy:                   []string{"service"},
			},
			CostAnalysisUI: usageapiv1beta1.UsageCarbonEmissionsQueryQueryDefinitionCostAnalysisUI{Graph: "BARS"},
		},
	}}
	manager := &UsageCarbonEmissionsQueryServiceManager{}
	hooks := newUsageCarbonEmissionsQueryRuntimeHooks(manager, sdkClient)
	client := wrapUsageCarbonEmissionsQueryGeneratedClient(hooks, defaultUsageCarbonEmissionsQueryServiceClient{ServiceClient: generatedruntime.NewServiceClient[*usageapiv1beta1.UsageCarbonEmissionsQuery](buildUsageCarbonEmissionsQueryGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*usageapiv1beta1.UsageCarbonEmissionsQuery]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *usageapiv1beta1.UsageCarbonEmissionsQuery) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *usageapiv1beta1.UsageCarbonEmissionsQuery) error {
			if current.Status.QueryDefinition.DisplayName != resource.Spec.QueryDefinition.DisplayName {
				return fmt.Errorf("created status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *usageapiv1beta1.UsageCarbonEmissionsQuery) {
			current.Spec.QueryDefinition.DisplayName = "osok-replay-carbon-query-updated"
		},
		ValidateUpdated: func(current *usageapiv1beta1.UsageCarbonEmissionsQuery) error {
			if current.Status.QueryDefinition.DisplayName != "osok-replay-carbon-query-updated" {
				return fmt.Errorf("updated status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredUsageCarbonEmissionsQueryEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
