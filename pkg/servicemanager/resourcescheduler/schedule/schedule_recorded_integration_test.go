/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package schedule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	resourceschedulersdk "github.com/oracle/oci-go-sdk/v65/resourcescheduler"
	resourceschedulerv1beta1 "github.com/oracle/oci-service-operator/api/resourcescheduler/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

const recordedScheduleName = "osok-replay-resource-schedule-v1"

func TestRecordedScheduleCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "resourcescheduler", Resource: "Schedule", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenResourceSchedulerSDK(t, mode, filepath.Join("testdata", "recordings", "schedule_crud.yaml"), metadata)
	compartmentID, resourceID := "ocid1.compartment.oc1..replay", "ocid1.instance.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredScheduleRecordingEnv(t, "OCI_COMPARTMENT_ID")
		resourceID = requiredScheduleRecordingEnv(t, "OCI_REPLAY_SCHEDULE_RESOURCE_ID")
	}
	resource := &resourceschedulerv1beta1.Schedule{Spec: resourceschedulerv1beta1.ScheduleSpec{
		CompartmentId: compartmentID, DisplayName: recordedScheduleName, Description: "recorded create",
		Action: string(resourceschedulersdk.ScheduleActionStartResource), RecurrenceDetails: "FREQ=DAILY;INTERVAL=1", RecurrenceType: string(resourceschedulersdk.ScheduleRecurrenceTypeIcal),
		Resources: []resourceschedulerv1beta1.ScheduleResource{{Id: resourceID}}, TimeStarts: "2099-01-01T00:00:00Z", FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	hooks := newScheduleRuntimeHooksWithOCIClient(sdkClient)
	applyScheduleRuntimeHooks(&hooks)
	manager := &ScheduleServiceManager{Log: loggerutil.OSOKLogger{}}
	client := wrapScheduleGeneratedClient(hooks, defaultScheduleServiceClient{ServiceClient: generatedruntime.NewServiceClient[*resourceschedulerv1beta1.Schedule](buildScheduleGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*resourceschedulerv1beta1.Schedule]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *resourceschedulerv1beta1.Schedule) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *resourceschedulerv1beta1.Schedule) error {
			if current.Status.DisplayName != recordedScheduleName || current.Status.Id == "" {
				return fmt.Errorf("created Schedule status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *resourceschedulerv1beta1.Schedule) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *resourceschedulerv1beta1.Schedule) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated Schedule status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredScheduleRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
