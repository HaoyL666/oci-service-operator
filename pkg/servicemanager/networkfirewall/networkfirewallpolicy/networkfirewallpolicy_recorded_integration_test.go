/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package networkfirewallpolicy

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

func TestRecordedNetworkFirewallPolicyCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkfirewall", Resource: "NetworkFirewallPolicy", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredNetworkFirewallPolicyEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkFirewallSDK(t, mode, filepath.Join("testdata", "recordings", "networkfirewallpolicy_crud.yaml"), metadata)
	resource := &networkfirewallv1beta1.NetworkFirewallPolicy{Spec: networkfirewallv1beta1.NetworkFirewallPolicySpec{CompartmentId: compartmentID, DisplayName: "osok-replay-network-firewall-policy", Description: "OSOK recorded Network Firewall policy", FreeformTags: map[string]string{"osok-replay": "create"}}}
	manager := &NetworkFirewallPolicyServiceManager{}
	hooks := newNetworkFirewallPolicyRuntimeHooks(manager, sdkClient)
	client := wrapNetworkFirewallPolicyGeneratedClient(hooks, defaultNetworkFirewallPolicyServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.NetworkFirewallPolicy](buildNetworkFirewallPolicyGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*networkfirewallv1beta1.NetworkFirewallPolicy]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *networkfirewallv1beta1.NetworkFirewallPolicy) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *networkfirewallv1beta1.NetworkFirewallPolicy) error {
			if current.Status.DisplayName != "osok-replay-network-firewall-policy" {
				return fmt.Errorf("created NetworkFirewallPolicy status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *networkfirewallv1beta1.NetworkFirewallPolicy) {
			current.Spec.DisplayName = "osok-replay-network-firewall-policy-updated"
			current.Spec.Description = "OSOK recorded Network Firewall policy updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *networkfirewallv1beta1.NetworkFirewallPolicy) error {
			if current.Status.DisplayName != "osok-replay-network-firewall-policy-updated" || current.Status.Description != "OSOK recorded Network Firewall policy updated" {
				return fmt.Errorf("updated NetworkFirewallPolicy status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredNetworkFirewallPolicyEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
