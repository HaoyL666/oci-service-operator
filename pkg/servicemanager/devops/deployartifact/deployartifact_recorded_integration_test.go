/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package deployartifact

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	devopssdk "github.com/oracle/oci-go-sdk/v65/devops"
	devopsv1beta1 "github.com/oracle/oci-service-operator/api/devops/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedDeployArtifactCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "devops",
		Resource: "DeployArtifact",
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
	repositoryID := "ocid1.artifactrepository.oc1..replay"
	if mode == ocireplay.ModeRecord {
		projectID = requiredDeployArtifactRecordingEnv(t, "OCI_REPLAY_DEVOPS_PROJECT_ID")
		repositoryID = requiredDeployArtifactRecordingEnv(t, "OCI_REPLAY_GENERIC_REPOSITORY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenDevOpsSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "deployartifact_crud.yaml"),
		metadata,
	)
	resource := &devopsv1beta1.DeployArtifact{
		Spec: devopsv1beta1.DeployArtifactSpec{
			DeployArtifactType:       string(devopssdk.DeployArtifactDeployArtifactTypeKubernetesManifest),
			ArgumentSubstitutionMode: string(devopssdk.DeployArtifactArgumentSubstitutionModeNone),
			ProjectId:                projectID,
			DisplayName:              "osok-replay-deploy-artifact",
			Description:              "OSOK recorded deploy artifact",
			DeployArtifactSource: devopsv1beta1.DeployArtifactSource{
				DeployArtifactSourceType: string(devopssdk.DeployArtifactSourceDeployArtifactSourceTypeGenericArtifact),
				RepositoryId:             repositoryID,
				DeployArtifactPath:       "manifests/app.yaml",
				DeployArtifactVersion:    "1.0.0",
			},
			FreeformTags: map[string]string{"osok-replay": "create"},
		},
	}
	client := newDeployArtifactServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*devopsv1beta1.DeployArtifact]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  10 * time.Second,
		Timeout:       20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		RetryError:    func(err error) bool { return strings.Contains(err.Error(), "NotAuthorizedOrNotFound") },
		HasIdentity: func(current *devopsv1beta1.DeployArtifact) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *devopsv1beta1.DeployArtifact) error {
			if current.Status.DisplayName != "osok-replay-deploy-artifact" {
				return fmt.Errorf("created DeployArtifact status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *devopsv1beta1.DeployArtifact) {
			current.Spec.Description = "OSOK recorded deploy artifact updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *devopsv1beta1.DeployArtifact) error {
			if current.Status.Description != "OSOK recorded deploy artifact updated" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated DeployArtifact status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredDeployArtifactRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
