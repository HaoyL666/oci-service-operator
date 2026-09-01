/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package mounttarget

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	filestoragesdk "github.com/oracle/oci-go-sdk/v65/filestorage"
	filestoragev1beta1 "github.com/oracle/oci-service-operator/api/filestorage/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedMountTargetName = "osok-replay-common-mount-target-v1"

func TestRecordedMountTargetCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedMountTargetSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close MountTarget cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	availabilityDomain := "example:US-ASHBURN-AD-1"
	subnetID := "ocid1.subnet.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredMountTargetRecordingEnv(t, "OCI_COMPARTMENT_ID")
		availabilityDomain = requiredMountTargetRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN")
		subnetID = requiredMountTargetRecordingEnv(t, "OCI_FILE_STORAGE_SUBNET_ID")
	}
	resource := &filestoragev1beta1.MountTarget{
		Spec: filestoragev1beta1.MountTargetSpec{
			AvailabilityDomain: availabilityDomain,
			CompartmentId:      compartmentID,
			SubnetId:           subnetID,
			DisplayName:        recordedMountTargetName,
			HostnameLabel:      "osokmtv1",
			FreeformTags:       map[string]string{"osok-replay": "create"},
		},
	}
	client := newRecordedMountTargetClient(sdkClient)
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
			_ = ocireplay.Await(cleanupCtx, mode, 15*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitMountTargetConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(filestoragesdk.MountTargetLifecycleStateActive) ||
		resource.Status.DisplayName != recordedMountTargetName ||
		resource.Status.ExportSetId == "" {
		t.Fatalf("created MountTarget status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedMountTargetName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitMountTargetConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated MountTarget status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, mountTargetPollInterval(mode), func() (bool, error) {
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

func newRecordedMountTargetClient(
	sdkClient filestoragesdk.FileStorageClient,
) MountTargetServiceClient {
	manager := &MountTargetServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}
	hooks := newMountTargetRuntimeHooks(manager, sdkClient)
	delegate := defaultMountTargetServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*filestoragev1beta1.MountTarget](
			buildMountTargetGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return wrapMountTargetGeneratedClient(hooks, delegate)
}

func openRecordedMountTargetSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (filestoragesdk.FileStorageClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service:  "filestorage",
		Resource: "MountTarget",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "mounttarget_crud.yaml")
	bindings := map[string]string{
		"availability-domain": "example:US-ASHBURN-AD-1",
		"subnet":              "ocid1.subnet.oc1..replay",
	}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := filestoragesdk.NewFileStorageClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		bindings = map[string]string{
			"availability-domain": requiredMountTargetRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN"),
			"subnet":              requiredMountTargetRecordingEnv(t, "OCI_FILE_STORAGE_SUBNET_ID"),
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path:       cassettePath,
			Metadata:   metadata,
			BaseClient: &client.BaseClient,
			Bindings:   bindings,
			Overwrite:  ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}

	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path:     cassettePath,
		Host:     "https://filestorage.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20171215",
		Metadata: metadata,
		Bindings: bindings,
	})
	if err != nil {
		t.Fatal(err)
	}
	return filestoragesdk.FileStorageClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitMountTargetConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client MountTargetServiceClient,
	resource *filestoragev1beta1.MountTarget,
) error {
	return ocireplay.Await(ctx, mode, mountTargetPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("MountTarget reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func mountTargetPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 15 * time.Second
	}
	return 0
}

func requiredMountTargetRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
