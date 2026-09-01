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

	devopssdk "github.com/oracle/oci-go-sdk/v65/devops"
	devopsv1beta1 "github.com/oracle/oci-service-operator/api/devops/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedRepositoryName = "osok-replay-repository-v1"

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

	projectID := "ocid1.devopsproject.oc1..replay"
	if mode == ocireplay.ModeRecord {
		projectID = requiredRepositoryRecordingEnv(t, "OCI_REPLAY_DEVOPS_PROJECT_ID")
	}
	resource := &devopsv1beta1.Repository{Spec: devopsv1beta1.RepositorySpec{
		Name:           recordedRepositoryName,
		ProjectId:      projectID,
		RepositoryType: string(devopssdk.RepositoryRepositoryTypeHosted),
		DefaultBranch:  "main",
		Description:    "recorded create",
		FreeformTags:   map[string]string{"osok-replay": "create"},
	}}
	client := newRecordedRepositoryClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 10*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitRepositoryConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(devopssdk.RepositoryLifecycleStateActive) ||
		resource.Status.Name != recordedRepositoryName ||
		resource.Status.ProjectId != projectID {
		t.Fatalf("created Repository status = %+v", resource.Status)
	}

	resource.Spec.Description = "recorded update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitRepositoryConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Description != resource.Spec.Description ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Repository status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
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

func newRecordedRepositoryClient(sdkClient devopssdk.DevopsClient) RepositoryServiceClient {
	manager := &RepositoryServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}
	hooks := newRepositoryDefaultRuntimeHooks(sdkClient)
	applyRepositoryRuntimeHooks(&hooks, sdkClient, nil)
	delegate := defaultRepositoryServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*devopsv1beta1.Repository](
			buildRepositoryGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return wrapRepositoryGeneratedClient(hooks, delegate)
}

func openRecordedRepositorySDK(
	t *testing.T,
	mode ocireplay.Mode,
) (devopssdk.DevopsClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "devops",
		Resource: "Repository",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "repository_crud.yaml")

	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := devopssdk.NewDevopsClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path:       cassettePath,
			Metadata:   metadata,
			BaseClient: &client.BaseClient,
			Overwrite:  ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}

	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path:     cassettePath,
		Host:     "https://devops.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20210630",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return devopssdk.DevopsClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitRepositoryConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client RepositoryServiceClient,
	resource *devopsv1beta1.Repository,
) error {
	return ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
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

func requiredRepositoryRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
