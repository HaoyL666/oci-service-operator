/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package networkfirewall

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	networkfirewallv1beta1 "github.com/oracle/oci-service-operator/api/networkfirewall/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestSyntheticNetworkFirewallCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "networkfirewall", Resource: "NetworkFirewall", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	sdkClient, closeSession := ocireplay.OpenNetworkFirewallSDK(t, ocireplay.ModeReplay, filepath.Join("testdata", "recordings", "networkfirewall_synthetic_crud.yaml"), metadata)
	resource := &networkfirewallv1beta1.NetworkFirewall{Spec: networkfirewallv1beta1.NetworkFirewallSpec{CompartmentId: "ocid1.compartment.oc1..replay", SubnetId: "ocid1.subnet.oc1..replay", NetworkFirewallPolicyId: "ocid1.networkfirewallpolicy.oc1..replay", DisplayName: "osok-replay-network-firewall", Shape: "NETWORK_FIREWALL_SMALL", FreeformTags: map[string]string{"osok-replay": "create"}}}
	manager := &NetworkFirewallServiceManager{}
	hooks := newNetworkFirewallRuntimeHooks(manager, sdkClient)
	client := wrapNetworkFirewallGeneratedClient(hooks, defaultNetworkFirewallServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.NetworkFirewall](buildNetworkFirewallGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*networkfirewallv1beta1.NetworkFirewall]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: closeSession, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *networkfirewallv1beta1.NetworkFirewall) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *networkfirewallv1beta1.NetworkFirewall) error {
			if current.Status.DisplayName != "osok-replay-network-firewall" {
				return fmt.Errorf("created NetworkFirewall status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *networkfirewallv1beta1.NetworkFirewall) {
			current.Spec.DisplayName = "osok-replay-network-firewall-updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *networkfirewallv1beta1.NetworkFirewall) error {
			if current.Status.DisplayName != "osok-replay-network-firewall-updated" {
				return fmt.Errorf("updated NetworkFirewall status = %+v", current.Status)
			}
			return nil
		},
	})
}
