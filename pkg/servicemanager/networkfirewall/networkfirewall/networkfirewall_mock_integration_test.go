/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package networkfirewall

import (
	"path/filepath"
	"testing"

	networkfirewallsdk "github.com/oracle/oci-go-sdk/v65/networkfirewall"
	networkfirewallv1beta1 "github.com/oracle/oci-service-operator/api/networkfirewall/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationNetworkFirewallLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "networkfirewall_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close NetworkFirewall OCI mock: %v", err)
		}
	})
	resource := &networkfirewallv1beta1.NetworkFirewall{Spec: networkfirewallv1beta1.NetworkFirewallSpec{CompartmentId: "ocid1.compartment.oc1..replay", SubnetId: "ocid1.subnet.oc1..replay", NetworkFirewallPolicyId: "ocid1.networkfirewallpolicy.oc1..replay", DisplayName: "osok-replay-network-firewall", Shape: "NETWORK_FIREWALL_SMALL", FreeformTags: map[string]string{"osok-replay": "create"}}}
	ocimock.InitializeResource(resource, "mock-networkfirewall")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := networkfirewallsdk.
		NetworkFirewallClient{BaseClient: session.BaseClient()}
	manager := &NetworkFirewallServiceManager{}
	hooks := newNetworkFirewallRuntimeHooks(manager, sdkClient)
	client := wrapNetworkFirewallGeneratedClient(hooks, defaultNetworkFirewallServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.NetworkFirewall](buildNetworkFirewallGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
