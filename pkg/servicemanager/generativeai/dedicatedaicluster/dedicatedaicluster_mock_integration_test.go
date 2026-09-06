/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package dedicatedaicluster

import (
	"path/filepath"
	"testing"

	generativeaisdk "github.com/oracle/oci-go-sdk/v65/generativeai"
	generativeaiv1beta1 "github.com/oracle/oci-service-operator/api/generativeai/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationDedicatedAiClusterLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "dedicatedaicluster_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close DedicatedAiCluster OCI mock: %v", err)
		}
	})
	resource := &generativeaiv1beta1.DedicatedAiCluster{
		Spec: generativeaiv1beta1.DedicatedAiClusterSpec{
			Type:          string(generativeaisdk.DedicatedAiClusterTypeHosting),
			CompartmentId: "ocid1.compartment.oc1..replay",
			UnitCount:     1,
			UnitShape: string(
				generativeaisdk.DedicatedAiClusterUnitShapeSmallCohere,
			),
			DisplayName: syntheticDedicatedAiClusterName,
			Description: "synthetic create",
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	ocimock.InitializeResource(resource, "mock-dedicatedaicluster")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := generativeaisdk.GenerativeAiClient{BaseClient: session.BaseClient()}
	client := newSyntheticDedicatedAiClusterClient(sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
