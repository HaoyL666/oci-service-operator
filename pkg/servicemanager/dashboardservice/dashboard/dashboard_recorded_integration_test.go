/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dashboard

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	dashboardservicesdk "github.com/oracle/oci-go-sdk/v65/dashboardservice"
	dashboardservicev1beta1 "github.com/oracle/oci-service-operator/api/dashboardservice/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedDashboardName = "osok-replay-dashboard-v1"

func TestRecordedDashboardCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedDashboardSDK(t, mode)
	dashboardGroupID := "ocid1.consoledashboardgroup.oc1..replay"
	if mode == ocireplay.ModeRecord {
		dashboardGroupID = requiredDashboardRecordingEnv(t, "OCI_REPLAY_DASHBOARD_GROUP_ID")
	}
	resource := &dashboardservicev1beta1.Dashboard{Spec: dashboardservicev1beta1.DashboardSpec{
		DashboardGroupId: dashboardGroupID, SchemaVersion: "V1", DisplayName: recordedDashboardName,
		Description: "recorded create", FreeformTags: map[string]string{"osok-replay": "create"},
		Config:  dashboardJSONValue(`{"layout":"grid"}`),
		Widgets: []shared.JSONValue{dashboardJSONValue(`{"name":"recorded-widget"}`)},
	}}
	client := newDashboardServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*dashboardservicev1beta1.Dashboard]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *dashboardservicev1beta1.Dashboard) bool { return current.Status.Id != "" },
		ValidateCreated: func(current *dashboardservicev1beta1.Dashboard) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedDashboardName {
				return fmt.Errorf("created Dashboard status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *dashboardservicev1beta1.Dashboard) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *dashboardservicev1beta1.Dashboard) error {
			if current.Status.Description != current.Spec.Description || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated Dashboard status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedDashboardSDK(t *testing.T, mode ocireplay.Mode) (dashboardservicesdk.DashboardClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "dashboardservice", Resource: "Dashboard", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "dashboard_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := dashboardservicesdk.NewDashboardClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://dashboard.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210731", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return dashboardservicesdk.DashboardClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredDashboardRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
