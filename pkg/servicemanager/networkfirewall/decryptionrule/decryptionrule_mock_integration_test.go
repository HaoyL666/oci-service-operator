/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package decryptionrule

import (
	"path/filepath"
	"testing"

	networkfirewallsdk "github.com/oracle/oci-go-sdk/v65/networkfirewall"
	networkfirewallv1beta1 "github.com/oracle/oci-service-operator/api/networkfirewall/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// Contract evidence: recorded composite-path CRUD, production service manager, and real OCI SDK serialization.
func TestMockIntegrationDecryptionRuleCompositeCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "decryptionrule_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close DecryptionRule OCI mock: %v", err)
		}
	})
	resource := &networkfirewallv1beta1.DecryptionRule{}
	ocimock.InitializeResource(resource, "mock-decryptionrule")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	resource.Spec.NetworkFirewallPolicyId = "<ocid:1>"
	sdkClient := networkfirewallsdk.NetworkFirewallClient{BaseClient: session.BaseClient()}
	manager := &DecryptionRuleServiceManager{}
	hooks := newDecryptionRuleRuntimeHooks(manager, sdkClient)
	client := wrapDecryptionRuleGeneratedClient(hooks, defaultDecryptionRuleServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.DecryptionRule](buildDecryptionRuleGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
