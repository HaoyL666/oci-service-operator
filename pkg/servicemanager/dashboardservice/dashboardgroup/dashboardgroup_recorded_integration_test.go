/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dashboardgroup

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
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedDashboardGroupName = "osok-replay-dashboard-group-v1"

func TestRecordedDashboardGroupCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedDashboardGroupSDK(t, mode)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredDashboardGroupRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &dashboardservicev1beta1.DashboardGroup{Spec: dashboardservicev1beta1.DashboardGroupSpec{
		CompartmentId: compartmentID, DisplayName: recordedDashboardGroupName,
		Description: "recorded create", FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	client := newDashboardGroupServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*dashboardservicev1beta1.DashboardGroup]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *dashboardservicev1beta1.DashboardGroup) bool { return current.Status.Id != "" },
		ValidateCreated: func(current *dashboardservicev1beta1.DashboardGroup) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedDashboardGroupName {
				return fmt.Errorf("created DashboardGroup status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *dashboardservicev1beta1.DashboardGroup) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *dashboardservicev1beta1.DashboardGroup) error {
			if current.Status.Description != current.Spec.Description || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated DashboardGroup status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedDashboardGroupSDK(t *testing.T, mode ocireplay.Mode) (dashboardservicesdk.DashboardGroupClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "dashboardservice", Resource: "DashboardGroup", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "dashboardgroup_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := dashboardservicesdk.NewDashboardGroupClientWithConfigurationProvider(provider)
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
	return dashboardservicesdk.DashboardGroupClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredDashboardGroupRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
