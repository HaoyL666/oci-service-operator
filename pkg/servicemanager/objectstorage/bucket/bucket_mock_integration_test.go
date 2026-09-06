/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package bucket

import (
	"path/filepath"
	"testing"

	objectstoragesdk "github.com/oracle/oci-go-sdk/v65/objectstorage"
	objectstoragev1beta1 "github.com/oracle/oci-service-operator/api/objectstorage/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: recorded composite-path CRUD, production service manager, and real OCI SDK serialization.
func TestMockIntegrationBucketCompositeCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "bucket_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Bucket OCI mock: %v", err)
		}
	})
	resource := &objectstoragev1beta1.Bucket{}
	ocimock.InitializeResource(resource, "mock-bucket")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	resource.Spec.Namespace = "<binding:objectstorage-namespace>"
	sdkClient := objectstoragesdk.ObjectStorageClient{BaseClient: session.BaseClient()}
	manager := &BucketServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newBucketDefaultRuntimeHooks(sdkClient)
	applyBucketRuntimeHooks(&hooks)
	client := defaultBucketServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*objectstoragev1beta1.Bucket](buildBucketGeneratedRuntimeConfig(manager, hooks)),
	}
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
