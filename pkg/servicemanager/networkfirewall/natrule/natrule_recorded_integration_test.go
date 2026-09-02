/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package natrule

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

func TestRecordedNatRuleCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkfirewall", Resource: "NatRule", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	policyID := "ocid1.networkfirewallpolicy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		policyID = requiredNatRuleEnv(t, "OCI_REPLAY_NETWORK_FIREWALL_POLICY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkFirewallSDK(t, mode, filepath.Join("testdata", "recordings", "natrule_crud.yaml"), metadata)
	resource := &networkfirewallv1beta1.NatRule{Spec: networkfirewallv1beta1.NatRuleSpec{NetworkFirewallPolicyId: policyID, Name: "osok_replay_nat_rule", Type: "NATV4", Condition: networkfirewallv1beta1.NatRuleCondition{SourceAddress: []string{"osok_replay_prereq_addresses"}, DestinationAddress: []string{"osok_replay_prereq_addresses"}, Service: "osok_replay_prereq_tcp"}, Action: "DIPP_SRC_NAT"}}
	manager := &NatRuleServiceManager{}
	hooks := newNatRuleRuntimeHooks(manager, sdkClient)
	client := wrapNatRuleGeneratedClient(hooks, defaultNatRuleServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.NatRule](buildNatRuleGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*networkfirewallv1beta1.NatRule]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 2 * time.Second, Timeout: 10 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *networkfirewallv1beta1.NatRule) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *networkfirewallv1beta1.NatRule) error {
			if current.Status.Name != current.Spec.Name {
				return fmt.Errorf("created NatRule status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *networkfirewallv1beta1.NatRule) {
			current.Spec.Condition.DestinationAddress = []string{"osok_replay_prereq_addresses", "osok_replay_addresses2"}
		},
		ValidateUpdated: func(current *networkfirewallv1beta1.NatRule) error {
			if len(current.Status.Condition.DestinationAddress) != 2 {
				return fmt.Errorf("updated NatRule status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredNatRuleEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
