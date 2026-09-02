/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package applicationgroup

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

func TestRecordedApplicationGroupCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkfirewall", Resource: "ApplicationGroup", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	policyID := "ocid1.networkfirewallpolicy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		policyID = requiredApplicationGroupEnv(t, "OCI_REPLAY_NETWORK_FIREWALL_POLICY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkFirewallSDK(t, mode, filepath.Join("testdata", "recordings", "applicationgroup_crud.yaml"), metadata)
	resource := &networkfirewallv1beta1.ApplicationGroup{Spec: networkfirewallv1beta1.ApplicationGroupSpec{NetworkFirewallPolicyId: policyID, Name: "osok_replay_app_group", Apps: []string{"osok_replay_prereq_icmp"}}}
	manager := &ApplicationGroupServiceManager{}
	hooks := newApplicationGroupRuntimeHooks(manager, sdkClient)
	client := wrapApplicationGroupGeneratedClient(hooks, defaultApplicationGroupServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.ApplicationGroup](buildApplicationGroupGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*networkfirewallv1beta1.ApplicationGroup]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 2 * time.Second, Timeout: 10 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *networkfirewallv1beta1.ApplicationGroup) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *networkfirewallv1beta1.ApplicationGroup) error {
			if current.Status.Name != current.Spec.Name {
				return fmt.Errorf("created ApplicationGroup status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *networkfirewallv1beta1.ApplicationGroup) {
			current.Spec.Apps = []string{"osok_replay_prereq_icmp", "osok_replay_icmp2"}
		},
		ValidateUpdated: func(current *networkfirewallv1beta1.ApplicationGroup) error {
			if len(current.Status.Apps) != 2 {
				return fmt.Errorf("updated ApplicationGroup status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredApplicationGroupEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
