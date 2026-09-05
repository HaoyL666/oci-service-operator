/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package batchtaskprofile

import (
	"path/filepath"
	"testing"

	batchsdk "github.com/oracle/oci-go-sdk/v65/batch"
	batchv1beta1 "github.com/oracle/oci-service-operator/api/batch/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationBatchTaskProfileEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "batchtaskprofile_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close BatchTaskProfile OCI mock: %v", err)
		}
	})
	resource := &batchv1beta1.BatchTaskProfile{}
	ocimock.InitializeResource(resource, "mock-batchtaskprofile")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := batchsdk.BatchComputingClient{BaseClient: session.BaseClient()}
	manager := &BatchTaskProfileServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newBatchTaskProfileRuntimeHooks(manager, sdkClient)
	client := wrapBatchTaskProfileGeneratedClient(hooks, defaultBatchTaskProfileServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*batchv1beta1.BatchTaskProfile](buildBatchTaskProfileGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
