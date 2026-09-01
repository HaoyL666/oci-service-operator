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

	aidocumentsdk "github.com/oracle/oci-go-sdk/v65/aidocument"
	aidocumentv1beta1 "github.com/oracle/oci-service-operator/api/aidocument/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedProjectName = "osok-replay-ai-document-project-v1"

func TestRecordedProjectCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedProjectSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Project cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredProjectRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &aidocumentv1beta1.Project{Spec: aidocumentv1beta1.ProjectSpec{
		CompartmentId: compartmentID,
		DisplayName:   recordedProjectName,
		Description:   "recorded create",
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	client := newProjectServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
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

	if err := awaitRecordedProject(generatedruntime.WithSkipExistingBeforeCreate(ctx), mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Id == "" || resource.Status.DisplayName != recordedProjectName {
		t.Fatalf("created Project status = %+v", resource.Status)
	}
	resource.Spec.Description = "recorded update"
	if err := awaitRecordedProject(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Description != resource.Spec.Description {
		t.Fatalf("updated Project status = %+v", resource.Status)
	}
	if err := ocireplay.Await(ctx, mode, recordedProjectPollInterval(mode), func() (bool, error) {
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

func openRecordedProjectSDK(t *testing.T, mode ocireplay.Mode) (aidocumentsdk.AIServiceDocumentClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "aidocument", Resource: "Project",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "project_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := aidocumentsdk.NewAIServiceDocumentClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://document.aiservice.us-ashburn-1.oci.oraclecloud.com", BasePath: "20221109", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return aidocumentsdk.AIServiceDocumentClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitRecordedProject(ctx context.Context, mode ocireplay.Mode, client ProjectServiceClient, resource *aidocumentv1beta1.Project) error {
	return ocireplay.Await(ctx, mode, recordedProjectPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Project reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func recordedProjectPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 5 * time.Second
	}
	return 0
}

func requiredProjectRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
