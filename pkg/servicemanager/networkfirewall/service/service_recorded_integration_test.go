/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package service

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

func TestRecordedServiceCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkfirewall", Resource: "Service", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	policyID := "ocid1.networkfirewallpolicy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		policyID = requiredServiceEnv(t, "OCI_REPLAY_NETWORK_FIREWALL_POLICY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkFirewallSDK(t, mode, filepath.Join("testdata", "recordings", "service_crud.yaml"), metadata)
	resource := &networkfirewallv1beta1.Service{Spec: networkfirewallv1beta1.ServiceSpec{NetworkFirewallPolicyId: policyID, Name: "osok_replay_service", Type: "TCP_SERVICE", PortRanges: []networkfirewallv1beta1.ServicePortRange{{MinimumPort: 8080, MaximumPort: 8080}}, Description: "OSOK recorded service"}}
	manager := &ServiceServiceManager{}
	hooks := newServiceRuntimeHooks(manager, sdkClient)
	client := wrapServiceGeneratedClient(hooks, defaultServiceServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.Service](buildServiceGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*networkfirewallv1beta1.Service]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 2 * time.Second, Timeout: 10 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *networkfirewallv1beta1.Service) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *networkfirewallv1beta1.Service) error {
			if current.Status.Name != current.Spec.Name {
				return fmt.Errorf("created Service status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *networkfirewallv1beta1.Service) {
			current.Spec.Description = "OSOK recorded Service updated"
		},
		ValidateUpdated: func(current *networkfirewallv1beta1.Service) error {
			if current.Status.Description != "OSOK recorded Service updated" {
				return fmt.Errorf("updated Service status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredServiceEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
