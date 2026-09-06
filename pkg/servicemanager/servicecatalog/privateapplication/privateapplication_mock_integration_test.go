/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package privateapplication

import (
	"path/filepath"
	"testing"

	servicecatalogsdk "github.com/oracle/oci-go-sdk/v65/servicecatalog"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationPrivateApplicationLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "privateapplication_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close PrivateApplication OCI mock: %v", err)
		}
	})
	resource := makePrivateApplicationResource()
	ocimock.InitializeResource(resource, "mock-privateapplication")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := servicecatalogsdk.ServiceCatalogClient{BaseClient: session.BaseClient()}
	client := newPrivateApplicationServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
