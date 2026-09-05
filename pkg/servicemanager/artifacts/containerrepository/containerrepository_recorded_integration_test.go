/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package containerrepository

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	artifactssdk "github.com/oracle/oci-go-sdk/v65/artifacts"
	artifactsv1beta1 "github.com/oracle/oci-service-operator/api/artifacts/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestRecordedContainerRepositoryCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedContainerRepositorySDK(t, mode)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = os.Getenv("OCI_COMPARTMENT_ID")
	}
	resource := &artifactsv1beta1.ContainerRepository{Spec: artifactsv1beta1.ContainerRepositorySpec{
		CompartmentId: compartmentID, DisplayName: "osok-replay-common-repository-v1",
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	client := newContainerRepositoryServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: logr.Discard()}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*artifactsv1beta1.ContainerRepository]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  5 * time.Second,
		Timeout:       10 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *artifactsv1beta1.ContainerRepository) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *artifactsv1beta1.ContainerRepository) error {
			if current.Status.Id == "" ||
				current.Status.LifecycleState != string(artifactssdk.ContainerRepositoryLifecycleStateAvailable) ||
				current.Status.DisplayName != current.Spec.DisplayName ||
				current.Status.IsPublic ||
				current.Status.FreeformTags["osok-replay"] != "create" {
				return fmt.Errorf("created ContainerRepository status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *artifactsv1beta1.ContainerRepository) {
			current.Spec.IsPublic = true
			current.Spec.Readme = artifactsv1beta1.ContainerRepositoryReadme{
				Content: "AI Factory OCI replay lifecycle",
				Format:  "text/markdown",
			}
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *artifactsv1beta1.ContainerRepository) error {
			if !current.Status.IsPublic ||
				current.Status.Readme.Content != current.Spec.Readme.Content ||
				current.Status.Readme.Format != string(artifactssdk.ContainerRepositoryReadmeFormatMarkdown) ||
				current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated ContainerRepository status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedContainerRepositorySDK(
	t *testing.T,
	mode ocireplay.Mode,
) (artifactssdk.ArtifactsClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service:  "artifacts",
		Resource: "ContainerRepository",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "containerrepository_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := artifactssdk.NewArtifactsClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(
			ocireplay.SDKRecordOptions{
				Path:       path,
				Metadata:   metadata,
				BaseClient: &client.BaseClient,
				Overwrite:  ocireplay.RecordingOverwriteRequested(),
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(
		ocireplay.SDKReplayOptions{
			Path:     path,
			Host:     "https://artifacts.us-ashburn-1.oci.oraclecloud.com",
			BasePath: "20160918",
			Metadata: metadata,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return artifactssdk.ArtifactsClient{BaseClient: session.BaseClient()}, session.Close
}
