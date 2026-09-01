/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	filestoragesdk "github.com/oracle/oci-go-sdk/v65/filestorage"
	filestoragev1beta1 "github.com/oracle/oci-service-operator/api/filestorage/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	recordedExportFileSystemName  = "osok-replay-export-filesystem-v1"
	recordedExportMountTargetName = "osok-replay-export-mount-target-v1"
	recordedExportPath            = "/osok-replay-v1"
)

func TestRecordedExportCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedExportSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Export cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	availabilityDomain := "example:US-ASHBURN-AD-1"
	subnetID := "ocid1.subnet.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredExportRecordingEnv(t, "OCI_COMPARTMENT_ID")
		availabilityDomain = requiredExportRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN")
		subnetID = requiredExportRecordingEnv(t, "OCI_FILE_STORAGE_SUBNET_ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	fileSystemID, err := createRecordedExportFileSystem(ctx, mode, sdkClient, compartmentID, availabilityDomain)
	if err != nil {
		t.Fatal(err)
	}
	mountTargetID, exportSetID, err := createRecordedExportMountTarget(ctx, mode, sdkClient, compartmentID, availabilityDomain, subnetID)
	if err != nil {
		_ = deleteRecordedExportFileSystem(ctx, mode, sdkClient, fileSystemID)
		t.Fatal(err)
	}
	prerequisitesDeleted := false
	t.Cleanup(func() {
		if prerequisitesDeleted || mode != ocireplay.ModeRecord {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cleanupCancel()
		_ = deleteRecordedExportMountTarget(cleanupCtx, mode, sdkClient, mountTargetID)
		_ = deleteRecordedExportFileSystem(cleanupCtx, mode, sdkClient, fileSystemID)
	})

	resource := &filestoragev1beta1.Export{Spec: filestoragev1beta1.ExportSpec{
		ExportSetId:  exportSetID,
		FileSystemId: fileSystemID,
		Path:         recordedExportPath,
		ExportOptions: []filestoragev1beta1.ExportOption{{
			Source: "10.0.0.0/16", RequirePrivilegedSourcePort: false,
			Access: "READ_WRITE", IdentitySquash: "NONE", AnonymousUid: 65534,
			AnonymousGid: 65534, AllowedAuth: []string{"SYS"},
		}},
	}}
	client := newRecordedExportClient(sdkClient)
	exportDeleted := false
	t.Cleanup(func() {
		if exportDeleted || mode != ocireplay.ModeRecord || resource.Status.Id == "" {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cleanupCancel()
		_ = ocireplay.Await(cleanupCtx, mode, 5*time.Second, func() (bool, error) {
			return client.Delete(cleanupCtx, resource)
		})
	})

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitExportConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(filestoragesdk.ExportLifecycleStateActive) ||
		resource.Status.Path != recordedExportPath {
		t.Fatalf("created Export status = %+v", resource.Status)
	}

	resource.Spec.ExportOptions[0].Access = "READ_ONLY"
	if err := awaitExportConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if len(resource.Status.ExportOptions) != 1 || resource.Status.ExportOptions[0].Access != "READ_ONLY" {
		t.Fatalf("updated Export status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, exportPollInterval(mode), func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	exportDeleted = true
	if err := deleteRecordedExportMountTarget(ctx, mode, sdkClient, mountTargetID); err != nil {
		t.Fatal(err)
	}
	if err := deleteRecordedExportFileSystem(ctx, mode, sdkClient, fileSystemID); err != nil {
		t.Fatal(err)
	}
	prerequisitesDeleted = true
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func newRecordedExportClient(sdkClient filestoragesdk.FileStorageClient) ExportServiceClient {
	manager := &ExportServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newExportRuntimeHooks(manager, sdkClient)
	delegate := defaultExportServiceClient{ServiceClient: generatedruntime.NewServiceClient[*filestoragev1beta1.Export](buildExportGeneratedRuntimeConfig(manager, hooks))}
	return wrapExportGeneratedClient(hooks, delegate)
}

func createRecordedExportFileSystem(ctx context.Context, mode ocireplay.Mode, client filestoragesdk.FileStorageClient, compartmentID, availabilityDomain string) (string, error) {
	response, err := client.CreateFileSystem(ctx, filestoragesdk.CreateFileSystemRequest{CreateFileSystemDetails: filestoragesdk.CreateFileSystemDetails{
		AvailabilityDomain: common.String(availabilityDomain), CompartmentId: common.String(compartmentID), DisplayName: common.String(recordedExportFileSystemName),
	}})
	if err != nil {
		return "", err
	}
	id := stringValue(response.Id)
	err = ocireplay.Await(ctx, mode, exportPollInterval(mode), func() (bool, error) {
		got, err := client.GetFileSystem(ctx, filestoragesdk.GetFileSystemRequest{FileSystemId: common.String(id)})
		return err == nil && got.LifecycleState == filestoragesdk.FileSystemLifecycleStateActive, err
	})
	return id, err
}

func createRecordedExportMountTarget(ctx context.Context, mode ocireplay.Mode, client filestoragesdk.FileStorageClient, compartmentID, availabilityDomain, subnetID string) (string, string, error) {
	response, err := client.CreateMountTarget(ctx, filestoragesdk.CreateMountTargetRequest{CreateMountTargetDetails: filestoragesdk.CreateMountTargetDetails{
		AvailabilityDomain: common.String(availabilityDomain), CompartmentId: common.String(compartmentID), SubnetId: common.String(subnetID),
		DisplayName: common.String(recordedExportMountTargetName), HostnameLabel: common.String("osokexpv1"),
	}})
	if err != nil {
		return "", "", err
	}
	id := stringValue(response.Id)
	exportSetID := stringValue(response.ExportSetId)
	err = ocireplay.Await(ctx, mode, exportPollInterval(mode), func() (bool, error) {
		got, err := client.GetMountTarget(ctx, filestoragesdk.GetMountTargetRequest{MountTargetId: common.String(id)})
		if err == nil && got.ExportSetId != nil {
			exportSetID = *got.ExportSetId
		}
		return err == nil && got.LifecycleState == filestoragesdk.MountTargetLifecycleStateActive, err
	})
	return id, exportSetID, err
}

func deleteRecordedExportMountTarget(ctx context.Context, mode ocireplay.Mode, client filestoragesdk.FileStorageClient, id string) error {
	_, err := client.DeleteMountTarget(ctx, filestoragesdk.DeleteMountTargetRequest{MountTargetId: common.String(id)})
	if err != nil && !isRecordedExportNotFound(err) {
		return err
	}
	return ocireplay.Await(ctx, mode, exportPollInterval(mode), func() (bool, error) {
		response, err := client.GetMountTarget(ctx, filestoragesdk.GetMountTargetRequest{MountTargetId: common.String(id)})
		if isRecordedExportNotFound(err) {
			return true, nil
		}
		return err == nil && response.LifecycleState == filestoragesdk.MountTargetLifecycleStateDeleted, err
	})
}

func deleteRecordedExportFileSystem(ctx context.Context, mode ocireplay.Mode, client filestoragesdk.FileStorageClient, id string) error {
	_, err := client.DeleteFileSystem(ctx, filestoragesdk.DeleteFileSystemRequest{FileSystemId: common.String(id)})
	if err != nil && !isRecordedExportNotFound(err) {
		return err
	}
	return ocireplay.Await(ctx, mode, exportPollInterval(mode), func() (bool, error) {
		response, err := client.GetFileSystem(ctx, filestoragesdk.GetFileSystemRequest{FileSystemId: common.String(id)})
		if isRecordedExportNotFound(err) {
			return true, nil
		}
		return err == nil && response.LifecycleState == filestoragesdk.FileSystemLifecycleStateDeleted, err
	})
}

func isRecordedExportNotFound(err error) bool {
	if err == nil {
		return false
	}
	serviceErr, ok := common.IsServiceError(err)
	return ok && serviceErr.GetHTTPStatusCode() == 404
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func openRecordedExportSDK(t *testing.T, mode ocireplay.Mode) (filestoragesdk.FileStorageClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "filestorage", Resource: "Export", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	cassettePath := filepath.Join("testdata", "recordings", "export_crud.yaml")
	bindings := map[string]string{"availability-domain": "example:US-ASHBURN-AD-1", "subnet": "ocid1.subnet.oc1..replay"}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := filestoragesdk.NewFileStorageClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		bindings = map[string]string{"availability-domain": requiredExportRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN"), "subnet": requiredExportRecordingEnv(t, "OCI_FILE_STORAGE_SUBNET_ID")}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: cassettePath, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: cassettePath, Host: "https://filestorage.us-ashburn-1.oci.oraclecloud.com", BasePath: "20171215", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return filestoragesdk.FileStorageClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitExportConvergence(ctx context.Context, mode ocireplay.Mode, client ExportServiceClient, resource *filestoragev1beta1.Export) error {
	return ocireplay.Await(ctx, mode, exportPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Export reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func exportPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 5 * time.Second
	}
	return 0
}

func requiredExportRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
