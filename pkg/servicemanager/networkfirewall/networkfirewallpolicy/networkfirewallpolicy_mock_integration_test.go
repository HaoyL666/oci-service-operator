/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package networkfirewallpolicy

import (
	"path/filepath"
	"testing"

	networkfirewallsdk "github.com/oracle/oci-go-sdk/v65/networkfirewall"
	networkfirewallv1beta1 "github.com/oracle/oci-service-operator/api/networkfirewall/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationNetworkFirewallPolicyEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "networkfirewallpolicy_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close NetworkFirewallPolicy OCI mock: %v", err)
		}
	})
	resource := &networkfirewallv1beta1.NetworkFirewallPolicy{}
	ocimock.InitializeResource(resource, "mock-networkfirewallpolicy")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := networkfirewallsdk.NetworkFirewallClient{BaseClient: session.BaseClient()}
	manager := &NetworkFirewallPolicyServiceManager{}
	hooks := newNetworkFirewallPolicyRuntimeHooks(manager, sdkClient)
	client := wrapNetworkFirewallPolicyGeneratedClient(hooks, defaultNetworkFirewallPolicyServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.NetworkFirewallPolicy](buildNetworkFirewallPolicyGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
