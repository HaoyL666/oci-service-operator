/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package scheduledjob

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	osmanagementhubsdk "github.com/oracle/oci-go-sdk/v65/osmanagementhub"
	osmanagementhubv1beta1 "github.com/oracle/oci-service-operator/api/osmanagementhub/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedScheduledJobCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "osmanagementhub", Resource: "ScheduledJob", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	compartmentID := "ocid1.compartment.oc1..replay"
	resourceUID := types.UID("replay-scheduled-job")
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredScheduledJobRecordingEnv(t, "OCI_COMPARTMENT_ID")
		resourceUID = types.UID(fmt.Sprintf("record-scheduled-job-%d", time.Now().UnixNano()))
	}
	sdkClient, closeSession := ocireplay.OpenOSManagementHubScheduledJobSDK(t, mode, filepath.Join("testdata", "recordings", "scheduledjob_crud.yaml"), metadata)
	resource := &osmanagementhubv1beta1.ScheduledJob{
		ObjectMeta: metav1.ObjectMeta{Name: "osok-replay-osmh-scheduled-job", Namespace: "default", UID: resourceUID},
		Spec: osmanagementhubv1beta1.ScheduledJobSpec{
			CompartmentId:         compartmentID,
			DisplayName:           "osok-replay-osmh-scheduled-job",
			Description:           "OSOK recorded future update job",
			ScheduleType:          string(osmanagementhubsdk.ScheduleTypesOnetime),
			TimeNextExecution:     "2030-01-15T10:00:00Z",
			Operations:            []osmanagementhubv1beta1.ScheduledJobOperation{{OperationType: string(osmanagementhubsdk.OperationTypesUpdateAll)}},
			ManagedCompartmentIds: []string{compartmentID},
			FreeformTags:          map[string]string{"osok-replay": "create"},
		},
	}
	client := newScheduledJobServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*osmanagementhubv1beta1.ScheduledJob]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *osmanagementhubv1beta1.ScheduledJob) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *osmanagementhubv1beta1.ScheduledJob) error {
			if current.Status.DisplayName != "osok-replay-osmh-scheduled-job" || current.Status.Id == "" {
				return fmt.Errorf("created ScheduledJob status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *osmanagementhubv1beta1.ScheduledJob) {
			current.Spec.DisplayName = "osok-replay-osmh-scheduled-job-updated"
			current.Spec.Description = "OSOK recorded future update job updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *osmanagementhubv1beta1.ScheduledJob) error {
			if current.Status.DisplayName != "osok-replay-osmh-scheduled-job-updated" || current.Status.Description != "OSOK recorded future update job updated" {
				return fmt.Errorf("updated ScheduledJob status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredScheduledJobRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
