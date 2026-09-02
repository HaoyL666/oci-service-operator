/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package managementdashboard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	managementdashboardv1beta1 "github.com/oracle/oci-service-operator/api/managementdashboard/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedManagementDashboardCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "managementdashboard",
		Resource: "ManagementDashboard",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	savedSearchID := "ocid1.managementsavedsearch.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredManagementDashboardEnv(t, "OCI_COMPARTMENT_ID")
		savedSearchID = requiredManagementDashboardEnv(t, "OCI_REPLAY_MANAGEMENT_SAVED_SEARCH_ID")
	}
	sdkClient, closeSession := ocireplay.OpenManagementDashboardSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "managementdashboard_crud.yaml"),
		metadata,
	)
	resource := &managementdashboardv1beta1.ManagementDashboard{
		Spec: managementdashboardv1beta1.ManagementDashboardSpec{
			ProviderId:      "log-analytics",
			ProviderName:    "Logging Analytics",
			ProviderVersion: "3.0.0",
			Tiles: []managementdashboardv1beta1.ManagementDashboardTile{{
				DisplayName:     "OSOK replay tile",
				SavedSearchId:   savedSearchID,
				Row:             1,
				Column:          1,
				Height:          4,
				Width:           6,
				Nls:             replayDashboardJSON(`{"title":"OSOK replay tile"}`),
				UiConfig:        replayDashboardJSON(`{"visualization":"table"}`),
				DataConfig:      []shared.JSONValue{replayDashboardJSON(`{"query":"*"}`)},
				State:           "DEFAULT",
				DrilldownConfig: replayDashboardJSON(`[]`),
				ParametersMap:   replayDashboardJSON(`{}`),
			}},
			DisplayName:       "osok-replay-management-dashboard",
			Description:       "OSOK recorded management dashboard",
			CompartmentId:     compartmentID,
			IsOobDashboard:    false,
			IsShowInHome:      false,
			MetadataVersion:   "2.0",
			IsShowDescription: true,
			ScreenImage:       "none",
			Nls:               replayDashboardJSON(`{"title":"OSOK replay dashboard"}`),
			UiConfig:          replayDashboardJSON(`{"layout":"grid"}`),
			DataConfig:        []shared.JSONValue{replayDashboardJSON(`{"source":"log-analytics"}`)},
			ParametersConfig:  []shared.JSONValue{},
			FeaturesConfig:    replayDashboardJSON(`{"crossService":{"shared":false}}`),
			DrilldownConfig:   []shared.JSONValue{},
			Type:              "NORMAL",
			IsFavorite:        false,
			FreeformTags:      map[string]string{"osok-replay": "create"},
		},
	}
	client := newManagementDashboardServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*managementdashboardv1beta1.ManagementDashboard]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  5 * time.Second,
		Timeout:       15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *managementdashboardv1beta1.ManagementDashboard) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *managementdashboardv1beta1.ManagementDashboard) error {
			if current.Status.DisplayName != "osok-replay-management-dashboard" || current.Status.LifecycleState != "ACTIVE" {
				return fmt.Errorf("created ManagementDashboard status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *managementdashboardv1beta1.ManagementDashboard) {
			current.Spec.Description = "OSOK recorded management dashboard updated"
			current.Spec.IsFavorite = true
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *managementdashboardv1beta1.ManagementDashboard) error {
			if current.Status.Description != "OSOK recorded management dashboard updated" || !current.Status.IsFavorite {
				return fmt.Errorf("updated ManagementDashboard status = %+v", current.Status)
			}
			return nil
		},
	})
}

func replayDashboardJSON(value string) shared.JSONValue {
	return shared.JSONValue{Raw: []byte(value)}
}

func requiredManagementDashboardEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
