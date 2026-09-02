/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	filestoragev1beta1 "github.com/oracle/oci-service-operator/api/filestorage/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestRecordedSnapshotCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "filestorage", Resource: "Snapshot", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	fileSystemID := "ocid1.filesystem.oc1..replay"
	if mode == ocireplay.ModeRecord {
		fileSystemID = requiredSnapshotRecordingEnv(t, "OCI_REPLAY_FILE_SYSTEM_ID")
	}
	sdkClient, closeSession := ocireplay.OpenFileStorageSDK(t, mode, filepath.Join("testdata", "recordings", "snapshot_crud.yaml"), metadata)
	resource := &filestoragev1beta1.Snapshot{Spec: filestoragev1beta1.SnapshotSpec{
		FileSystemId: fileSystemID,
		Name:         "osok-replay-snapshot",
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	manager := &SnapshotServiceManager{}
	hooks := newSnapshotRuntimeHooks(manager, sdkClient)
	client := wrapSnapshotGeneratedClient(hooks, defaultSnapshotServiceClient{ServiceClient: generatedruntime.NewServiceClient[*filestoragev1beta1.Snapshot](buildSnapshotGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*filestoragev1beta1.Snapshot]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute, CleanupTimeout: 2 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *filestoragev1beta1.Snapshot) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *filestoragev1beta1.Snapshot) error {
			if current.Status.Name != "osok-replay-snapshot" || current.Status.Id == "" {
				return fmt.Errorf("created Snapshot status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *filestoragev1beta1.Snapshot) {
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *filestoragev1beta1.Snapshot) error {
			if current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated Snapshot status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredSnapshotRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
