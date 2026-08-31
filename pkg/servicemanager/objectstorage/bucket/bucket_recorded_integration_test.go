/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package bucket

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	objectstoragesdk "github.com/oracle/oci-go-sdk/v65/objectstorage"
	objectstoragev1beta1 "github.com/oracle/oci-service-operator/api/objectstorage/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedBucketName = "osok-replay-basic-bucket-v1"

func TestRecordedBucketCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedBucketSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Bucket cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	namespace := "replaynamespace"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredRecordingEnv(t, "OCI_COMPARTMENT_ID")
		namespace = requiredRecordingEnv(t, "OCI_OBJECTSTORAGE_NAMESPACE")
	}
	resource := &objectstoragev1beta1.Bucket{
		Spec: objectstoragev1beta1.BucketSpec{
			Name:             recordedBucketName,
			CompartmentId:    compartmentID,
			Namespace:        namespace,
			PublicAccessType: "NoPublicAccess",
			StorageTier:      "Standard",
			Metadata:         map[string]string{"phase": "create"},
			FreeformTags:     map[string]string{"osok-replay": "create"},
		},
	}
	manager := &BucketServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newBucketDefaultRuntimeHooks(sdkClient)
	applyBucketRuntimeHooks(&hooks)
	client := defaultBucketServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*objectstoragev1beta1.Bucket](buildBucketGeneratedRuntimeConfig(manager, hooks)),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 5*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitBucketConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Name != resource.Spec.Name || resource.Status.Namespace != resource.Spec.Namespace || resource.Status.Metadata["phase"] != "create" {
		t.Fatalf("created Bucket status = %+v", resource.Status)
	}

	resource.Spec.Metadata = map[string]string{"phase": "update"}
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitBucketConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Metadata["phase"] != "update" || resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Bucket status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
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

func openRecordedBucketSDK(t *testing.T, mode ocireplay.Mode) (objectstoragesdk.ObjectStorageClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service:    "objectstorage",
		Resource:   "Bucket",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "bucket_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := objectstoragesdk.NewObjectStorageClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path:       path,
			Metadata:   metadata,
			BaseClient: &client.BaseClient,
			Bindings:   map[string]string{"objectstorage-namespace": requiredRecordingEnv(t, "OCI_OBJECTSTORAGE_NAMESPACE")},
			Overwrite:  ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path:     path,
		Host:     "https://objectstorage.us-ashburn-1.oraclecloud.com",
		BasePath: "n",
		Metadata: metadata,
		Bindings: map[string]string{"objectstorage-namespace": "replaynamespace"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return objectstoragesdk.ObjectStorageClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitBucketConvergence(ctx context.Context, mode ocireplay.Mode, client BucketServiceClient, resource *objectstoragev1beta1.Bucket) error {
	return ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Bucket reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
