/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package query

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

func TestRecordedQueryCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "usageapi", Resource: "Query", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	tenancyID := "ocid1.tenancy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		tenancyID = requiredUsageQueryEnv(t, "OCI_TENANCY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenUsageAPISDK(t, mode, filepath.Join("testdata", "recordings", "query_crud.yaml"), metadata)
	resource := &usageapiv1beta1.Query{Spec: usageapiv1beta1.QuerySpec{CompartmentId: tenancyID, QueryDefinition: usageapiv1beta1.QueryDefinition{DisplayName: "osok-replay-usage-query", Version: 1, ReportQuery: usageapiv1beta1.QueryDefinitionReportQuery{TenantId: tenancyID, Granularity: "MONTHLY", QueryType: "COST", GroupBy: []string{"service"}, TimeUsageStarted: "2026-08-01T00:00:00Z", TimeUsageEnded: "2026-09-01T00:00:00Z"}, CostAnalysisUI: usageapiv1beta1.QueryDefinitionCostAnalysisUI{Graph: "BARS"}}}}
	manager := &QueryServiceManager{}
	hooks := newQueryRuntimeHooks(manager, sdkClient)
	client := wrapQueryGeneratedClient(hooks, defaultQueryServiceClient{ServiceClient: generatedruntime.NewServiceClient[*usageapiv1beta1.Query](buildQueryGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*usageapiv1beta1.Query]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *usageapiv1beta1.Query) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *usageapiv1beta1.Query) error {
			if current.Status.QueryDefinition.DisplayName != resource.Spec.QueryDefinition.DisplayName {
				return fmt.Errorf("created status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *usageapiv1beta1.Query) {
			current.Spec.QueryDefinition.DisplayName = "osok-replay-usage-query-updated"
		},
		ValidateUpdated: func(current *usageapiv1beta1.Query) error {
			if current.Status.QueryDefinition.DisplayName != "osok-replay-usage-query-updated" {
				return fmt.Errorf("updated status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredUsageQueryEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
