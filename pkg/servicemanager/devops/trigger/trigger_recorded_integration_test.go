/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package trigger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	devopssdk "github.com/oracle/oci-go-sdk/v65/devops"
	devopsv1beta1 "github.com/oracle/oci-service-operator/api/devops/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedTriggerCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "devops",
		Resource: "Trigger",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	projectID := "ocid1.devopsproject.oc1..replay"
	repositoryID := "ocid1.devopsrepository.oc1..replay"
	buildPipelineID := "ocid1.devopsbuildpipeline.oc1..replay"
	if mode == ocireplay.ModeRecord {
		projectID = requiredTriggerRecordingEnv(t, "OCI_REPLAY_DEVOPS_PROJECT_ID")
		repositoryID = requiredTriggerRecordingEnv(t, "OCI_REPLAY_DEVOPS_REPOSITORY_ID")
		buildPipelineID = requiredTriggerRecordingEnv(t, "OCI_REPLAY_DEVOPS_BUILD_PIPELINE_ID")
	}
	sdkClient, closeSession := ocireplay.OpenDevOpsSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "trigger_crud.yaml"),
		metadata,
	)
	resource := &devopsv1beta1.Trigger{
		Spec: devopsv1beta1.TriggerSpec{
			JsonData:      fmt.Sprintf(`{"actions":[{"type":"TRIGGER_BUILD_PIPELINE","buildPipelineId":%q}]}`, buildPipelineID),
			DisplayName:   "osok-replay-devops-trigger",
			Description:   "OSOK recorded DevOps trigger",
			ProjectId:     projectID,
			TriggerSource: string(devopssdk.TriggerTriggerSourceDevopsCodeRepository),
			RepositoryId:  repositoryID,
			FreeformTags:  map[string]string{"osok-replay": "create"},
		},
	}
	client := newTriggerServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*devopsv1beta1.Trigger]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  10 * time.Second,
		Timeout:       20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *devopsv1beta1.Trigger) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *devopsv1beta1.Trigger) error {
			if current.Status.DisplayName != "osok-replay-devops-trigger" || current.Status.LifecycleState != string(devopssdk.TriggerLifecycleStateActive) {
				return fmt.Errorf("created Trigger status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *devopsv1beta1.Trigger) {
			current.Spec.Description = "OSOK recorded DevOps trigger updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *devopsv1beta1.Trigger) error {
			if current.Status.Description != "OSOK recorded DevOps trigger updated" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated Trigger status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredTriggerRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
