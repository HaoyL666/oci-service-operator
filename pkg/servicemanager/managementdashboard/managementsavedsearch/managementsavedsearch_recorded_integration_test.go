/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package managementsavedsearch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	managementdashboardsdk "github.com/oracle/oci-go-sdk/v65/managementdashboard"
	managementdashboardv1beta1 "github.com/oracle/oci-service-operator/api/managementdashboard/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedManagementSavedSearchCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "managementdashboard",
		Resource: "ManagementSavedSearch",
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
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredManagementSavedSearchEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := ocireplay.OpenManagementDashboardSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "managementsavedsearch_crud.yaml"),
		metadata,
	)
	resource := &managementdashboardv1beta1.ManagementSavedSearch{
		Spec: managementdashboardv1beta1.ManagementSavedSearchSpec{
			DisplayName:      "osok-replay-management-saved-search",
			ProviderId:       "log-analytics",
			ProviderVersion:  "3.0.0",
			ProviderName:     "Logging Analytics",
			CompartmentId:    compartmentID,
			IsOobSavedSearch: false,
			Description:      "OSOK recorded saved search",
			Nls:              replayManagementJSON(`{"title":"OSOK replay"}`),
			Type:             string(managementdashboardsdk.SavedSearchTypesWidgetShowInDashboard),
			UiConfig:         replayManagementJSON(`{"visualization":"table"}`),
			DataConfig:       []shared.JSONValue{replayManagementJSON(`{"query":"*"}`)},
			ScreenImage:      "none",
			MetadataVersion:  "2.0",
			WidgetTemplate:   "<div></div>",
			WidgetVM:         "{}",
			FreeformTags:     map[string]string{"osok-replay": "create"},
		},
	}
	client := newManagementSavedSearchServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*managementdashboardv1beta1.ManagementSavedSearch]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  5 * time.Second,
		Timeout:       15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		RetryError:    func(err error) bool { return strings.Contains(err.Error(), "NotAuthorizedOrNotFound") },
		HasIdentity: func(current *managementdashboardv1beta1.ManagementSavedSearch) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *managementdashboardv1beta1.ManagementSavedSearch) error {
			if current.Status.DisplayName != "osok-replay-management-saved-search" {
				return fmt.Errorf("created ManagementSavedSearch status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *managementdashboardv1beta1.ManagementSavedSearch) {
			current.Spec.Description = "OSOK recorded saved search updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *managementdashboardv1beta1.ManagementSavedSearch) error {
			if current.Status.Description != "OSOK recorded saved search updated" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated ManagementSavedSearch status = %+v", current.Status)
			}
			return nil
		},
	})
}

func replayManagementJSON(value string) shared.JSONValue {
	return shared.JSONValue{Raw: []byte(value)}
}

func requiredManagementSavedSearchEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
