/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package tunnelinspectionrule

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

func TestRecordedTunnelInspectionRuleCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkfirewall", Resource: "TunnelInspectionRule", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	policyID := "ocid1.networkfirewallpolicy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		policyID = requiredTunnelInspectionRuleEnv(t, "OCI_REPLAY_NETWORK_FIREWALL_POLICY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkFirewallSDK(t, mode, filepath.Join("testdata", "recordings", "tunnelinspectionrule_crud.yaml"), metadata)
	resource := &networkfirewallv1beta1.TunnelInspectionRule{Spec: networkfirewallv1beta1.TunnelInspectionRuleSpec{NetworkFirewallPolicyId: policyID, Name: "osok_replay_tunnel_rule", Protocol: "VXLAN", Condition: networkfirewallv1beta1.TunnelInspectionRuleCondition{SourceAddress: []string{"osok_replay_prereq_addresses"}, DestinationAddress: []string{"osok_replay_prereq_addresses"}}, Profile: networkfirewallv1beta1.TunnelInspectionRuleProfile{MustReturnTrafficToSource: true}, Action: "INSPECT"}}
	manager := &TunnelInspectionRuleServiceManager{}
	hooks := newTunnelInspectionRuleRuntimeHooks(manager, sdkClient)
	client := wrapTunnelInspectionRuleGeneratedClient(hooks, defaultTunnelInspectionRuleServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.TunnelInspectionRule](buildTunnelInspectionRuleGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*networkfirewallv1beta1.TunnelInspectionRule]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 2 * time.Second, Timeout: 10 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *networkfirewallv1beta1.TunnelInspectionRule) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *networkfirewallv1beta1.TunnelInspectionRule) error {
			if current.Status.Name != current.Spec.Name {
				return fmt.Errorf("created TunnelInspectionRule status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *networkfirewallv1beta1.TunnelInspectionRule) {
			current.Spec.Action = "INSPECT_AND_CAPTURE_LOG"
		},
		ValidateUpdated: func(current *networkfirewallv1beta1.TunnelInspectionRule) error {
			if current.Status.Action != "INSPECT_AND_CAPTURE_LOG" {
				return fmt.Errorf("updated TunnelInspectionRule status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredTunnelInspectionRuleEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
