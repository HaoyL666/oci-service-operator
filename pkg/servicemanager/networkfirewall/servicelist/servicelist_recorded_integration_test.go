/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package servicelist

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

func TestRecordedServiceListCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkfirewall", Resource: "ServiceList", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	policyID := "ocid1.networkfirewallpolicy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		policyID = requiredServiceListEnv(t, "OCI_REPLAY_NETWORK_FIREWALL_POLICY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkFirewallSDK(t, mode, filepath.Join("testdata", "recordings", "servicelist_crud.yaml"), metadata)
	resource := &networkfirewallv1beta1.ServiceList{Spec: networkfirewallv1beta1.ServiceListSpec{NetworkFirewallPolicyId: policyID, Name: "osok_replay_service_list", Services: []string{"osok_replay_prereq_tcp"}}}
	manager := &ServiceListServiceManager{}
	hooks := newServiceListRuntimeHooks(manager, sdkClient)
	client := wrapServiceListGeneratedClient(hooks, defaultServiceListServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.ServiceList](buildServiceListGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*networkfirewallv1beta1.ServiceList]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 2 * time.Second, Timeout: 10 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *networkfirewallv1beta1.ServiceList) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *networkfirewallv1beta1.ServiceList) error {
			if current.Status.Name != current.Spec.Name {
				return fmt.Errorf("created ServiceList status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *networkfirewallv1beta1.ServiceList) {
			current.Spec.Services = []string{"osok_replay_prereq_tcp", "osok_replay_udp2"}
		},
		ValidateUpdated: func(current *networkfirewallv1beta1.ServiceList) error {
			if len(current.Status.Services) != 2 {
				return fmt.Errorf("updated ServiceList status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredServiceListEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
