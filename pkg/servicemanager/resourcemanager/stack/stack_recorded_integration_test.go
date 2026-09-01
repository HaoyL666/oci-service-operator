/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package stack

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	resourcemanagersdk "github.com/oracle/oci-go-sdk/v65/resourcemanager"
	resourcemanagerv1beta1 "github.com/oracle/oci-service-operator/api/resourcemanager/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedStackName = "osok-replay-common-resource-manager-stack-v1"

func TestRecordedStackCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedStackSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Resource Manager Stack cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredStackRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &resourcemanagerv1beta1.Stack{Spec: resourcemanagerv1beta1.StackSpec{
		CompartmentId: compartmentID,
		ConfigSource: resourcemanagerv1beta1.StackConfigSource{
			ConfigSourceType:     "ZIP_UPLOAD",
			ZipFileBase64Encoded: recordedStackZip(t),
		},
		DisplayName:  recordedStackName,
		Description:  "recorded create",
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	client := newRecordedStackClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.Id == "" {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 10*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitStackConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(resourcemanagersdk.StackLifecycleStateActive) ||
		resource.Status.DisplayName != recordedStackName {
		t.Fatalf("created Stack status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedStackName + "-updated"
	resource.Spec.Description = "recorded update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitStackConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.Description != resource.Spec.Description ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Stack status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, stackPollInterval(mode), func() (bool, error) {
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

func recordedStackZip(t *testing.T) string {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create("main.tf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("terraform { required_version = \">= 1.0\" }\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buffer.Bytes())
}

func newRecordedStackClient(sdkClient resourcemanagersdk.ResourceManagerClient) StackServiceClient {
	manager := &StackServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newStackRuntimeHooks(manager, sdkClient)
	delegate := defaultStackServiceClient{ServiceClient: generatedruntime.NewServiceClient[*resourcemanagerv1beta1.Stack](buildStackGeneratedRuntimeConfig(manager, hooks))}
	return wrapStackGeneratedClient(hooks, delegate)
}

func openRecordedStackSDK(t *testing.T, mode ocireplay.Mode) (resourcemanagersdk.ResourceManagerClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "resourcemanager", Resource: "Stack", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	cassettePath := filepath.Join("testdata", "recordings", "stack_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := resourcemanagersdk.NewResourceManagerClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: cassettePath, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: cassettePath, Host: "https://resourcemanager.us-ashburn-1.oraclecloud.com", BasePath: "20180917", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return resourcemanagersdk.ResourceManagerClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitStackConvergence(ctx context.Context, mode ocireplay.Mode, client StackServiceClient, resource *resourcemanagerv1beta1.Stack) error {
	return ocireplay.Await(ctx, mode, stackPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Resource Manager Stack reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func stackPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 10 * time.Second
	}
	return 0
}

func requiredStackRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
