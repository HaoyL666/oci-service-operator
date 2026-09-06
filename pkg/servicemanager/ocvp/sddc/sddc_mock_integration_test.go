/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package sddc

import (
	"path/filepath"
	"testing"

	ocvpsdk "github.com/oracle/oci-go-sdk/v65/ocvp"
	ocvpv1beta1 "github.com/oracle/oci-service-operator/api/ocvp/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationSddcLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "sddc_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Sddc OCI mock: %v", err)
		}
	})
	resource := &ocvpv1beta1.Sddc{Spec: ocvpv1beta1.SddcSpec{
		VmwareSoftwareVersion: "8.0.2",
		CompartmentId:         "ocid1.compartment.oc1..replay",
		HcxMode:               string(ocvpsdk.HcxModesDisabled),
		InitialConfiguration: ocvpv1beta1.SddcInitialConfiguration{
			InitialClusterConfigurations: []ocvpv1beta1.SddcInitialConfigurationInitialClusterConfiguration{
				{
					VsphereType:               string(ocvpsdk.VsphereTypesManagement),
					ComputeAvailabilityDomain: "US-ASHBURN-AD-1",
					EsxiHostsCount:            3,
					DisplayName:               "management",
					InitialCommitment:         string(ocvpsdk.CommitmentHour),
					InitialHostShapeName:      "BM.DenseIO.E5.128",
					InitialHostOcpuCount:      128,
					NetworkConfiguration: ocvpv1beta1.SddcInitialConfigurationInitialClusterConfigurationNetworkConfiguration{
						ProvisioningSubnetId: "ocid1.subnet.oc1..replay",
						VmotionVlanId:        "ocid1.vlan.oc1..vmotion",
						VsanVlanId:           "ocid1.vlan.oc1..vsan",
						NsxVTepVlanId:        "ocid1.vlan.oc1..nsxvtep",
						NsxEdgeVTepVlanId:    "ocid1.vlan.oc1..nsxedgevtep",
						VsphereVlanId:        "ocid1.vlan.oc1..vsphere",
						NsxEdgeUplink1VlanId: "ocid1.vlan.oc1..uplink1",
						NsxEdgeUplink2VlanId: "ocid1.vlan.oc1..uplink2",
					},
				},
			},
		},
		SshAuthorizedKeys: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIReplayOnlyKey osok-replay",
		DisplayName:       syntheticSddcName,
		FreeformTags:      map[string]string{"osok-replay": "create"},
	}}
	ocimock.InitializeResource(resource, "mock-sddc")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := ocvpsdk.SddcClient{BaseClient: session.BaseClient()}
	manager := newSyntheticSddcManager(sdkClient)
	client := manager.client
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
