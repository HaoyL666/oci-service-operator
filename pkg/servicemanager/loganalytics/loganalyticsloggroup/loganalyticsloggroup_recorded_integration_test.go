/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package loganalyticsloggroup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	loganalyticsv1beta1 "github.com/oracle/oci-service-operator/api/loganalytics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedLogAnalyticsLogGroupName = "osok-replay-log-analytics-group-v1"

func TestRecordedLogAnalyticsLogGroupCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loganalytics", Resource: "LogAnalyticsLogGroup", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	compartmentID, namespaceName := "ocid1.compartment.oc1..replay", "replay-namespace"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredLogAnalyticsLogGroupRecordingEnv(t, "OCI_COMPARTMENT_ID")
		namespaceName = requiredLogAnalyticsLogGroupRecordingEnv(t, "OCI_REPLAY_LOG_ANALYTICS_NAMESPACE")
	}
	sdkClient, closeSession := ocireplay.OpenLogAnalyticsSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "loganalyticsloggroup_crud.yaml"),
		metadata,
		map[string]string{"loganalytics-namespace": namespaceName},
	)
	resource := &loganalyticsv1beta1.LogAnalyticsLogGroup{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName},
		Spec: loganalyticsv1beta1.LogAnalyticsLogGroupSpec{
			CompartmentId: compartmentID,
			DisplayName:   recordedLogAnalyticsLogGroupName,
			Description:   "recorded create",
			FreeformTags:  map[string]string{"osok-replay": "create"},
		},
	}
	client := newLogAnalyticsLogGroupServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*loganalyticsv1beta1.LogAnalyticsLogGroup]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *loganalyticsv1beta1.LogAnalyticsLogGroup) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *loganalyticsv1beta1.LogAnalyticsLogGroup) error {
			if current.Status.DisplayName != recordedLogAnalyticsLogGroupName || current.Status.Id == "" {
				return fmt.Errorf("created LogAnalyticsLogGroup status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *loganalyticsv1beta1.LogAnalyticsLogGroup) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *loganalyticsv1beta1.LogAnalyticsLogGroup) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated LogAnalyticsLogGroup status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredLogAnalyticsLogGroupRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
