/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package decryptionrule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	networkfirewallv1beta1 "github.com/oracle/oci-service-operator/api/networkfirewall/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestRecordedDecryptionRuleCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkfirewall", Resource: "DecryptionRule", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	policyID := "ocid1.networkfirewallpolicy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		policyID = requiredDecryptionRuleEnv(t, "OCI_REPLAY_NETWORK_FIREWALL_POLICY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkFirewallSDK(t, mode, filepath.Join("testdata", "recordings", "decryptionrule_crud.yaml"), metadata)
	resource := &networkfirewallv1beta1.DecryptionRule{Spec: networkfirewallv1beta1.DecryptionRuleSpec{NetworkFirewallPolicyId: policyID, Name: "osok_replay_decryption_rule", Condition: networkfirewallv1beta1.DecryptionRuleCondition{SourceAddress: []string{"osok_replay_prereq_addresses"}, DestinationAddress: []string{"osok_replay_prereq_addresses"}}, Action: "NO_DECRYPT"}}
	manager := &DecryptionRuleServiceManager{}
	hooks := newDecryptionRuleRuntimeHooks(manager, sdkClient)
	client := wrapDecryptionRuleGeneratedClient(hooks, defaultDecryptionRuleServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.DecryptionRule](buildDecryptionRuleGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*networkfirewallv1beta1.DecryptionRule]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 2 * time.Second, Timeout: 10 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *networkfirewallv1beta1.DecryptionRule) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *networkfirewallv1beta1.DecryptionRule) error {
			if current.Status.Name != current.Spec.Name {
				return fmt.Errorf("created DecryptionRule status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *networkfirewallv1beta1.DecryptionRule) {
			current.Spec.Condition.DestinationAddress = []string{"osok_replay_prereq_addresses", "osok_replay_addresses2"}
		},
		ValidateUpdated: func(current *networkfirewallv1beta1.DecryptionRule) error {
			if len(current.Status.Condition.DestinationAddress) != 2 {
				return fmt.Errorf("updated DecryptionRule status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredDecryptionRuleEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
