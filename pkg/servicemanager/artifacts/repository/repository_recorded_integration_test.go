/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package repository

import (
	"context"
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
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedRepositoryName = "osok-replay-generic-repository-v1"

func TestRecordedRepositoryCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedRepositorySDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Repository cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredRepositoryRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &artifactsv1beta1.Repository{Spec: artifactsv1beta1.RepositorySpec{
		CompartmentId:  compartmentID,
		DisplayName:    recordedRepositoryName,
		Description:    "recorded create",
		IsImmutable:    false,
		RepositoryType: "GENERIC",
		FreeformTags:   map[string]string{"osok-replay": "create"},
	}}
	client := newRepositoryServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: logr.Discard()}, sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.Id == "" {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 5*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	if err := awaitRecordedRepository(generatedruntime.WithSkipExistingBeforeCreate(ctx), mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != recordedRepositoryName || resource.Status.Id == "" {
		t.Fatalf("created Repository status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedRepositoryName + "-updated"
	resource.Spec.Description = "recorded update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitRecordedRepository(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName || resource.Status.Description != resource.Spec.Description {
		t.Fatalf("updated Repository status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, recordedRepositoryPollInterval(mode), func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	deleted = true
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func openRecordedRepositorySDK(t *testing.T, mode ocireplay.Mode) (artifactssdk.ArtifactsClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "artifacts", Resource: "Repository", Operations: []ocireplay.Operation{
		ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete,
	}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "repository_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := artifactssdk.NewArtifactsClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: path, Host: "https://artifacts.us-ashburn-1.oci.oraclecloud.com", BasePath: "20160918", Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return artifactssdk.ArtifactsClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitRecordedRepository(ctx context.Context, mode ocireplay.Mode, client RepositoryServiceClient, resource *artifactsv1beta1.Repository) error {
	return ocireplay.Await(ctx, mode, recordedRepositoryPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Repository reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func recordedRepositoryPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 5 * time.Second
	}
	return 0
}

func requiredRepositoryRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
