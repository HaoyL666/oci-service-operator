/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package cccupgradeschedule

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	computecloudatcustomersdk "github.com/oracle/oci-go-sdk/v65/computecloudatcustomer"
	computecloudatcustomerv1beta1 "github.com/oracle/oci-service-operator/api/computecloudatcustomer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedCccUpgradeScheduleName = "osok-replay-ccc-schedule"

func TestRecordedCccUpgradeScheduleCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredCccUpgradeScheduleEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := openRecordedCccUpgradeScheduleSDK(t, mode)
	resource := &computecloudatcustomerv1beta1.CccUpgradeSchedule{Spec: computecloudatcustomerv1beta1.CccUpgradeScheduleSpec{
		DisplayName: recordedCccUpgradeScheduleName, CompartmentId: compartmentID, Description: "recorded create",
		Events: []computecloudatcustomerv1beta1.CccUpgradeScheduleEvent{{
			Description: "OSOK replay maintenance window", TimeStart: "2026-09-05T00:00:00Z",
			ScheduleEventDuration: "PT49H", ScheduleEventRecurrences: "FREQ=MONTHLY",
		}},
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	manager := &CccUpgradeScheduleServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newCccUpgradeScheduleRuntimeHooks(manager, sdkClient)
	client := wrapCccUpgradeScheduleGeneratedClient(hooks, defaultCccUpgradeScheduleServiceClient{ServiceClient: generatedruntime.NewServiceClient[*computecloudatcustomerv1beta1.CccUpgradeSchedule](buildCccUpgradeScheduleGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*computecloudatcustomerv1beta1.CccUpgradeSchedule]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *computecloudatcustomerv1beta1.CccUpgradeSchedule) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *computecloudatcustomerv1beta1.CccUpgradeSchedule) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedCccUpgradeScheduleName {
				return fmt.Errorf("created CccUpgradeSchedule status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *computecloudatcustomerv1beta1.CccUpgradeSchedule) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *computecloudatcustomerv1beta1.CccUpgradeSchedule) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated CccUpgradeSchedule status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedCccUpgradeScheduleSDK(t *testing.T, mode ocireplay.Mode) (computecloudatcustomersdk.ComputeCloudAtCustomerClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "computecloudatcustomer", Resource: "CccUpgradeSchedule", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "cccupgradeschedule_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := computecloudatcustomersdk.NewComputeCloudAtCustomerClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://ccc.us-ashburn-1.oci.oraclecloud.com", BasePath: "20221208", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return computecloudatcustomersdk.ComputeCloudAtCustomerClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredCccUpgradeScheduleEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
