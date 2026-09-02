/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	dataflowv1beta1 "github.com/oracle/oci-service-operator/api/dataflow/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedApplicationCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "dataflow", Resource: "Application",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	fileURI := "oci://osok-replay@namespace/osok-replay.py"
	logsBucketURI := "oci://osok-replay@namespace/"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredApplicationEnv(t, "OCI_COMPARTMENT_ID")
		fileURI = requiredApplicationEnv(t, "OCI_REPLAY_DATAFLOW_FILE_URI")
		logsBucketURI = requiredApplicationEnv(t, "OCI_REPLAY_DATAFLOW_LOGS_URI")
	}
	sdkClient, closeSession := ocireplay.OpenDataFlowSDK(
		t, mode, filepath.Join("testdata", "recordings", "application_crud.yaml"), metadata,
	)
	resource := &dataflowv1beta1.Application{Spec: dataflowv1beta1.ApplicationSpec{
		CompartmentId: compartmentID, DisplayName: "osok-replay-data-flow-application",
		DriverShape: "VM.Standard.E4.Flex", ExecutorShape: "VM.Standard.E4.Flex",
		DriverShapeConfig:   dataflowv1beta1.ApplicationDriverShapeConfig{Ocpus: 1, MemoryInGBs: 16},
		ExecutorShapeConfig: dataflowv1beta1.ApplicationExecutorShapeConfig{Ocpus: 1, MemoryInGBs: 16},
		Language:            "PYTHON", NumExecutors: 1, SparkVersion: "3.5.0", FileUri: fileURI,
		LogsBucketUri: logsBucketURI,
		Type:          "BATCH", Description: "OSOK recorded Data Flow application",
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	client := newApplicationServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*dataflowv1beta1.Application]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *dataflowv1beta1.Application) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *dataflowv1beta1.Application) error {
			if current.Status.DisplayName != "osok-replay-data-flow-application" || current.Status.LifecycleState == "" {
				return fmt.Errorf("created Application status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *dataflowv1beta1.Application) {
			current.Spec.Description = "OSOK recorded Data Flow application updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *dataflowv1beta1.Application) error {
			if current.Status.Description != "OSOK recorded Data Flow application updated" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated Application status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredApplicationEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
