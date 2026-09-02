/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package addresslist

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

func TestRecordedAddressListCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkfirewall", Resource: "AddressList", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	policyID := "ocid1.networkfirewallpolicy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		policyID = requiredAddressListEnv(t, "OCI_REPLAY_NETWORK_FIREWALL_POLICY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkFirewallSDK(t, mode, filepath.Join("testdata", "recordings", "addresslist_crud.yaml"), metadata)
	resource := &networkfirewallv1beta1.AddressList{Spec: networkfirewallv1beta1.AddressListSpec{NetworkFirewallPolicyId: policyID, Name: "osok_replay_address_list", Type: "IP", Addresses: []string{"10.20.0.0/24"}, Description: "OSOK recorded address list"}}
	manager := &AddressListServiceManager{}
	hooks := newAddressListRuntimeHooks(manager, sdkClient)
	client := wrapAddressListGeneratedClient(hooks, defaultAddressListServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.AddressList](buildAddressListGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*networkfirewallv1beta1.AddressList]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 2 * time.Second, Timeout: 10 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *networkfirewallv1beta1.AddressList) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *networkfirewallv1beta1.AddressList) error {
			if current.Status.Name != resource.Spec.Name || len(current.Status.Addresses) != 1 {
				return fmt.Errorf("created AddressList status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *networkfirewallv1beta1.AddressList) {
			current.Spec.Addresses = []string{"10.20.0.0/24", "10.21.0.0/24"}
			current.Spec.Description = "OSOK recorded address list updated"
		},
		ValidateUpdated: func(current *networkfirewallv1beta1.AddressList) error {
			if len(current.Status.Addresses) != 2 || current.Status.Description != "OSOK recorded address list updated" {
				return fmt.Errorf("updated AddressList status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredAddressListEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
