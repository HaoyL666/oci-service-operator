/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package dataset

import (
	"path/filepath"
	"testing"

	datalabelingservicesdk "github.com/oracle/oci-go-sdk/v65/datalabelingservice"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationDatasetLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "dataset_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Dataset OCI mock: %v", err)
		}
	})
	resource := newDatasetTestResource()
	ocimock.InitializeResource(resource, "mock-dataset")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := datalabelingservicesdk.DataLabelingManagementClient{BaseClient: session.BaseClient()}
	client := newDatasetServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
