/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package rovercluster

import (
	"path/filepath"
	"testing"

	roversdk "github.com/oracle/oci-go-sdk/v65/rover"
	roverv1beta1 "github.com/oracle/oci-service-operator/api/rover/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationRoverClusterLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "rovercluster_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close RoverCluster OCI mock: %v", err)
		}
	})
	resource := &roverv1beta1.RoverCluster{
		Spec: roverv1beta1.RoverClusterSpec{
			DisplayName:   syntheticRoverClusterName,
			CompartmentId: "ocid1.compartment.oc1..replay",
			ClusterSize:   5,
			ClusterType:   "STANDALONE",
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	ocimock.InitializeResource(resource, "mock-rovercluster")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := roversdk.RoverClusterClient{BaseClient: session.BaseClient()}
	manager := newSyntheticRoverClusterManager(sdkClient)
	client := manager.client
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
