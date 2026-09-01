/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	datasciencesdk "github.com/oracle/oci-go-sdk/v65/datascience"
	datasciencev1beta1 "github.com/oracle/oci-service-operator/api/datascience/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedDataScienceProjectName = "osok-replay-common-data-science-project-v1"

func TestRecordedProjectCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedDataScienceProjectSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Data Science Project cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredDataScienceProjectRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &datasciencev1beta1.Project{Spec: datasciencev1beta1.ProjectSpec{
		CompartmentId: compartmentID,
		DisplayName:   recordedDataScienceProjectName,
		Description:   "recorded create",
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	client := newRecordedDataScienceProjectClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.Id == "" {
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
	if err := awaitDataScienceProjectConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(datasciencesdk.ProjectLifecycleStateActive) ||
		resource.Status.DisplayName != recordedDataScienceProjectName {
		t.Fatalf("created Data Science Project status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedDataScienceProjectName + "-updated"
	resource.Spec.Description = "recorded update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitDataScienceProjectConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.Description != resource.Spec.Description ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Data Science Project status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, dataScienceProjectPollInterval(mode), func() (bool, error) {
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

func newRecordedDataScienceProjectClient(
	sdkClient datasciencesdk.DataScienceClient,
) ProjectServiceClient {
	manager := &ProjectServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}
	hooks := newProjectRuntimeHooks(manager, sdkClient)
	delegate := defaultProjectServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datasciencev1beta1.Project](
			buildProjectGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return wrapProjectGeneratedClient(hooks, delegate)
}

func openRecordedDataScienceProjectSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (datasciencesdk.DataScienceClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service:  "datascience",
		Resource: "Project",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "project_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := datasciencesdk.NewDataScienceClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path: cassettePath, Metadata: metadata, BaseClient: &client.BaseClient,
			Overwrite: ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: cassettePath, Host: "https://datascience.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20190101", Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return datasciencesdk.DataScienceClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitDataScienceProjectConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client ProjectServiceClient,
	resource *datasciencev1beta1.Project,
) error {
	return ocireplay.Await(ctx, mode, dataScienceProjectPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Data Science Project reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func dataScienceProjectPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 10 * time.Second
	}
	return 0
}

func requiredDataScienceProjectRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
