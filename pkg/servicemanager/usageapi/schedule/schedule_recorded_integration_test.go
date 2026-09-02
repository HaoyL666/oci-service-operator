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

	usageapiv1beta1 "github.com/oracle/oci-service-operator/api/usageapi/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestRecordedScheduleCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "usageapi", Resource: "Schedule", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	tenancyID, namespace, bucketName := "ocid1.tenancy.oc1..replay", "replaynamespace", "replay-usage-bucket"
	const scheduledTime = "2030-01-15T10:00:00Z"
	resourceUID := types.UID("replay-usage-schedule")
	if mode == ocireplay.ModeRecord {
		tenancyID = requiredScheduleRecordingEnv(t, "OCI_TENANCY_ID")
		namespace = requiredScheduleRecordingEnv(t, "OCI_OBJECT_STORAGE_NAMESPACE")
		bucketName = requiredScheduleRecordingEnv(t, "OCI_REPLAY_USAGE_BUCKET_NAME")
		resourceUID = types.UID(fmt.Sprintf("record-usage-schedule-%d", time.Now().UnixNano()))
	}
	sdkClient, closeSession := ocireplay.OpenUsageAPISDK(t, mode, filepath.Join("testdata", "recordings", "schedule_crud.yaml"), metadata, map[string]string{
		"object-storage-namespace": namespace,
		"bucket-name":              bucketName,
	})
	resource := &usageapiv1beta1.Schedule{
		ObjectMeta: metav1.ObjectMeta{Name: "osok-replay-usage-schedule", Namespace: "default", UID: resourceUID},
		Spec: usageapiv1beta1.ScheduleSpec{
			Name:                "osok-replay-usage-schedule",
			CompartmentId:       tenancyID,
			ScheduleRecurrences: "DAILY",
			TimeScheduled:       scheduledTime,
			Description:         "OSOK recorded usage export",
			OutputFileFormat:    "CSV",
			ResultLocation: usageapiv1beta1.ScheduleResultLocation{
				LocationType: "OBJECT_STORAGE",
				Region:       "us-ashburn-1",
				Namespace:    namespace,
				BucketName:   bucketName,
			},
			QueryProperties: usageapiv1beta1.ScheduleQueryProperties{
				Granularity: "DAILY",
				DateRange: usageapiv1beta1.ScheduleQueryPropertiesDateRange{
					DateRangeType:        "DYNAMIC",
					DynamicDateRangeType: "LAST_7_DAYS",
				},
			},
			FreeformTags: map[string]string{"osok-replay": "create"},
		},
	}
	manager := &ScheduleServiceManager{}
	hooks := newScheduleRuntimeHooks(manager, sdkClient)
	client := wrapScheduleGeneratedClient(hooks, defaultScheduleServiceClient{ServiceClient: generatedruntime.NewServiceClient[*usageapiv1beta1.Schedule](buildScheduleGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*usageapiv1beta1.Schedule]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *usageapiv1beta1.Schedule) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *usageapiv1beta1.Schedule) error {
			if current.Status.Name != "osok-replay-usage-schedule" || current.Status.Id == "" {
				return fmt.Errorf("created Schedule status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *usageapiv1beta1.Schedule) {
			current.Spec.Description = "OSOK recorded usage export updated"
			current.Spec.OutputFileFormat = "PDF"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *usageapiv1beta1.Schedule) error {
			if current.Status.Description != "OSOK recorded usage export updated" || current.Status.OutputFileFormat != "PDF" {
				return fmt.Errorf("updated Schedule status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredScheduleRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
