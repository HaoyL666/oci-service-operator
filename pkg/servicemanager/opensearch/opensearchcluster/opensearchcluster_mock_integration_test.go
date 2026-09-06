/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package opensearchcluster

import (
	"path/filepath"
	"testing"

	opensearchsdk "github.com/oracle/oci-go-sdk/v65/opensearch"
	opensearchv1beta1 "github.com/oracle/oci-service-operator/api/opensearch/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationOpensearchClusterLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "opensearchcluster_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close OpensearchCluster OCI mock: %v", err)
		}
	})
	resource := &opensearchv1beta1.OpensearchCluster{Spec: opensearchv1beta1.OpensearchClusterSpec{
		DisplayName:                    syntheticOpensearchClusterName,
		CompartmentId:                  "ocid1.compartment.oc1..replay",
		SoftwareVersion:                "2.11.0",
		MasterNodeCount:                3,
		MasterNodeHostType:             string(opensearchsdk.MasterNodeHostTypeFlex),
		MasterNodeHostOcpuCount:        1,
		MasterNodeHostMemoryGB:         16,
		DataNodeCount:                  3,
		DataNodeHostType:               string(opensearchsdk.DataNodeHostTypeFlex),
		DataNodeHostOcpuCount:          2,
		DataNodeHostMemoryGB:           32,
		DataNodeStorageGB:              50,
		OpendashboardNodeCount:         1,
		OpendashboardNodeHostOcpuCount: 1,
		OpendashboardNodeHostMemoryGB:  8,
		VcnId:                          "ocid1.vcn.oc1..replay",
		SubnetId:                       "ocid1.subnet.oc1..replay",
		VcnCompartmentId:               "ocid1.compartment.oc1..replay",
		SubnetCompartmentId:            "ocid1.compartment.oc1..replay",
		SecurityMode:                   string(opensearchsdk.SecurityModeDisabled),
		FreeformTags:                   map[string]string{"osok-replay": "synthetic"},
	}}
	ocimock.InitializeResource(resource, "mock-opensearchcluster")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := opensearchsdk.OpensearchClusterClient{BaseClient: session.BaseClient()}
	client := newSyntheticOpensearchClusterClient(sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
