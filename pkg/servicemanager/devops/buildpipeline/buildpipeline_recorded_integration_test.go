/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package buildpipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	devopsv1beta1 "github.com/oracle/oci-service-operator/api/devops/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedBuildPipelineName = "osok-replay-build-pipeline-v1"

func TestRecordedBuildPipelineCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "devops", Resource: "BuildPipeline", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenDevOpsSDK(t, mode, filepath.Join("testdata", "recordings", "buildpipeline_crud.yaml"), metadata)
	projectID := "ocid1.devopsproject.oc1..replay"
	if mode == ocireplay.ModeRecord {
		projectID = requiredBuildPipelineRecordingEnv(t, "OCI_REPLAY_DEVOPS_PROJECT_ID")
	}
	resource := &devopsv1beta1.BuildPipeline{Spec: devopsv1beta1.BuildPipelineSpec{
		ProjectId:    projectID,
		DisplayName:  recordedBuildPipelineName,
		Description:  "recorded create",
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	client := newBuildPipelineServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*devopsv1beta1.BuildPipeline]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 30 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *devopsv1beta1.BuildPipeline) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *devopsv1beta1.BuildPipeline) error {
			if current.Status.DisplayName != recordedBuildPipelineName || current.Status.ProjectId != projectID {
				return fmt.Errorf("created BuildPipeline status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *devopsv1beta1.BuildPipeline) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *devopsv1beta1.BuildPipeline) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated BuildPipeline status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredBuildPipelineRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
