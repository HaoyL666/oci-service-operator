/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package filesystemsnapshotpolicy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	filestoragesdk "github.com/oracle/oci-go-sdk/v65/filestorage"
	filestoragev1beta1 "github.com/oracle/oci-service-operator/api/filestorage/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedFilesystemSnapshotPolicyName = "osok-replay-filesystem-snapshot-policy-v1"

func TestRecordedFilesystemSnapshotPolicyCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	availabilityDomain := "replay-availability-domain"
	if mode == ocireplay.ModeRecord {
		availabilityDomain = requiredFilesystemSnapshotPolicyRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN")
	}
	sdkClient, closeSession := openRecordedFilesystemSnapshotPolicySDK(t, mode, availabilityDomain)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close FilesystemSnapshotPolicy cassette: %v", err)
			}
		}
	})
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredFilesystemSnapshotPolicyRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &filestoragev1beta1.FilesystemSnapshotPolicy{Spec: filestoragev1beta1.FilesystemSnapshotPolicySpec{AvailabilityDomain: availabilityDomain, CompartmentId: compartmentID, DisplayName: recordedFilesystemSnapshotPolicyName, PolicyPrefix: "replay", FreeformTags: map[string]string{"osok-replay": "create"}}}
	manager := &FilesystemSnapshotPolicyServiceManager{Log: loggerutil.OSOKLogger{Logger: logr.Discard()}}
	hooks := newFilesystemSnapshotPolicyRuntimeHooks(manager, sdkClient)
	client := wrapFilesystemSnapshotPolicyGeneratedClient(hooks, defaultFilesystemSnapshotPolicyServiceClient{ServiceClient: generatedruntime.NewServiceClient[*filestoragev1beta1.FilesystemSnapshotPolicy](buildFilesystemSnapshotPolicyGeneratedRuntimeConfig(manager, hooks))})
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
			_ = ocireplay.Await(cleanupCtx, mode, 5*time.Second, func() (bool, error) { return client.Delete(cleanupCtx, resource) })
		})
	}
	if err := awaitRecordedFilesystemSnapshotPolicy(generatedruntime.WithSkipExistingBeforeCreate(ctx), mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName || resource.Status.Id == "" {
		t.Fatalf("created FilesystemSnapshotPolicy status = %+v", resource.Status)
	}
	resource.Spec.DisplayName = recordedFilesystemSnapshotPolicyName + "-updated"
	resource.Spec.PolicyPrefix = "updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitRecordedFilesystemSnapshotPolicy(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName || resource.Status.PolicyPrefix != resource.Spec.PolicyPrefix {
		t.Fatalf("updated FilesystemSnapshotPolicy status = %+v", resource.Status)
	}
	if err := ocireplay.Await(ctx, mode, recordedFilesystemSnapshotPolicyPollInterval(mode), func() (bool, error) { return client.Delete(ctx, resource) }); err != nil {
		t.Fatal(err)
	}
	deleted = true
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func openRecordedFilesystemSnapshotPolicySDK(t *testing.T, mode ocireplay.Mode, availabilityDomain string) (filestoragesdk.FileStorageClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "filestorage", Resource: "FilesystemSnapshotPolicy", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "filesystemsnapshotpolicy_crud.yaml")
	bindings := map[string]string{"availability-domain": availabilityDomain}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := filestoragesdk.NewFileStorageClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://filestorage.us-ashburn-1.oci.oraclecloud.com", BasePath: "20171215", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return filestoragesdk.FileStorageClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitRecordedFilesystemSnapshotPolicy(ctx context.Context, mode ocireplay.Mode, client FilesystemSnapshotPolicyServiceClient, resource *filestoragev1beta1.FilesystemSnapshotPolicy) error {
	return ocireplay.Await(ctx, mode, recordedFilesystemSnapshotPolicyPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("FilesystemSnapshotPolicy reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}
func recordedFilesystemSnapshotPolicyPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 5 * time.Second
	}
	return 0
}
func requiredFilesystemSnapshotPolicyRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
