/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package bucket

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	objectstoragesdk "github.com/oracle/oci-go-sdk/v65/objectstorage"
	objectstoragev1beta1 "github.com/oracle/oci-service-operator/api/objectstorage/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/errorutil"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticBucketCreateRecoversAfterThrottling(t *testing.T) {
	sdkClient, closeSession := openSyntheticBucketSDK(
		t,
		"bucket_throttle_then_success.yaml",
		[]ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead},
	)
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := closeSession(); err != nil {
				t.Errorf("close throttled Bucket cassette: %v", err)
			}
		}
	})

	resource := syntheticFailureBucket("osok-replay-synthetic-throttle-v1")
	client := newSyntheticBucketClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)

	response, err := client.CreateOrUpdate(createCtx, resource, ctrl.Request{})
	if err == nil {
		t.Fatal("first Bucket create error = nil, want OCI throttling error")
	}
	if response.IsSuccessful {
		t.Fatalf("first Bucket create response = %+v, want unsuccessful", response)
	}
	var throttlingErr errorutil.TooManyRequestsOciError
	if !errors.As(err, &throttlingErr) || throttlingErr.HTTPStatusCode != 429 {
		t.Fatalf("first Bucket create error = %v, want 429 TooManyRequests", err)
	}

	if err := awaitBucketConvergence(
		createCtx,
		ocireplay.ModeReplay,
		client,
		resource,
	); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Name != resource.Spec.Name ||
		resource.Status.FreeformTags["osok-replay"] != "retry" {
		t.Fatalf("recovered Bucket status = %+v", resource.Status)
	}

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func TestSyntheticBucketDeleteTreatsConfirmedNotFoundAsSuccess(t *testing.T) {
	sdkClient, closeSession := openSyntheticBucketSDK(
		t,
		"bucket_delete_not_found.yaml",
		[]ocireplay.Operation{ocireplay.OperationRead, ocireplay.OperationDelete},
	)
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := closeSession(); err != nil {
				t.Errorf("close absent Bucket cassette: %v", err)
			}
		}
	})

	resource := syntheticFailureBucket("osok-replay-synthetic-absent-v1")
	client := newSyntheticBucketClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	deleted, err := client.Delete(ctx, resource)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("Bucket Delete() = false, want confirmed absence to release deletion")
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("Bucket status.deletedAt = nil, want confirmed deletion timestamp")
	}

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func syntheticFailureBucket(name string) *objectstoragev1beta1.Bucket {
	return &objectstoragev1beta1.Bucket{
		Spec: objectstoragev1beta1.BucketSpec{
			Name:             name,
			CompartmentId:    "ocid1.compartment.oc1..replay",
			Namespace:        "replaynamespace",
			PublicAccessType: "NoPublicAccess",
			StorageTier:      "Standard",
			Metadata:         map[string]string{"phase": "retry"},
			FreeformTags:     map[string]string{"osok-replay": "retry"},
		},
	}
}

func newSyntheticBucketClient(
	sdkClient objectstoragesdk.ObjectStorageClient,
) BucketServiceClient {
	manager := &BucketServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")},
	}
	hooks := newBucketDefaultRuntimeHooks(sdkClient)
	applyBucketRuntimeHooks(&hooks)
	return defaultBucketServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*objectstoragev1beta1.Bucket](
			buildBucketGeneratedRuntimeConfig(manager, hooks),
		),
	}
}

func openSyntheticBucketSDK(
	t *testing.T,
	cassetteName string,
	operations []ocireplay.Operation,
) (objectstoragesdk.ObjectStorageClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:    "objectstorage",
		Resource:   "Bucket",
		Operations: operations,
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path:     filepath.Join("testdata", "recordings", cassetteName),
		Host:     "https://objectstorage.us-ashburn-1.oraclecloud.com",
		BasePath: "n",
		Metadata: metadata,
		Bindings: map[string]string{
			"objectstorage-namespace": "replaynamespace",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return objectstoragesdk.ObjectStorageClient{
		BaseClient: session.BaseClient(),
	}, session.Close
}
