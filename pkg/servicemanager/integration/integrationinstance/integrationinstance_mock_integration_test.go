/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package integrationinstance

import (
	"path/filepath"
	"testing"

	integrationsdk "github.com/oracle/oci-go-sdk/v65/integration"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationIntegrationInstanceLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "integrationinstance_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close IntegrationInstance OCI mock: %v", err)
		}
	})
	resource := newIntegrationInstanceTestResource()
	ocimock.InitializeResource(resource, "mock-integrationinstance")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := integrationsdk.IntegrationInstanceClient{BaseClient: session.BaseClient()}
	hooks := newIntegrationInstanceDefaultRuntimeHooks(sdkClient)
	applyIntegrationInstanceRuntimeHooks(&hooks)
	client := newIntegrationInstanceRuntimeTestClient(hooks)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
