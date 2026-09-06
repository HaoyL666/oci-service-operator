/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package odaprivateendpoint

import (
	"path/filepath"
	"testing"

	odasdk "github.com/oracle/oci-go-sdk/v65/oda"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationOdaPrivateEndpointLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "odaprivateendpoint_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close OdaPrivateEndpoint OCI mock: %v", err)
		}
	})
	resource := newOdaPrivateEndpointResource("osok-replay-oda-endpoint")
	ocimock.InitializeResource(resource, "mock-odaprivateendpoint")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := odasdk.ManagementClient{BaseClient: session.BaseClient()}
	client := newOdaPrivateEndpointServiceClientWithOCIClient(sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
