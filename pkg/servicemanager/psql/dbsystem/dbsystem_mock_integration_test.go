/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package dbsystem

import (
	"path/filepath"
	"testing"

	psqlsdk "github.com/oracle/oci-go-sdk/v65/psql"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationDbSystemLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "dbsystem_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close DbSystem OCI mock: %v", err)
		}
	})
	resource := testDbSystemResource()
	ocimock.InitializeResource(resource, "mock-dbsystem")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := psqlsdk.PostgresqlClient{BaseClient: session.BaseClient()}
	client := manualDbSystemServiceClient{sdk: sdkClient, log: discardDbSystemLogger()}
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
