/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package urllist

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

func TestRecordedUrlListCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "networkfirewall", Resource: "UrlList", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	policyID := "ocid1.networkfirewallpolicy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		policyID = requiredUrlListEnv(t, "OCI_REPLAY_NETWORK_FIREWALL_POLICY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenNetworkFirewallSDK(t, mode, filepath.Join("testdata", "recordings", "urllist_crud.yaml"), metadata)
	resource := &networkfirewallv1beta1.UrlList{Spec: networkfirewallv1beta1.UrlListSpec{NetworkFirewallPolicyId: policyID, Name: "osok_replay_url_list", Urls: []networkfirewallv1beta1.UrlListUrl{{Type: "SIMPLE", Pattern: "*.example.com/*"}}, Description: "OSOK recorded URL list"}}
	manager := &UrlListServiceManager{}
	hooks := newUrlListRuntimeHooks(manager, sdkClient)
	client := wrapUrlListGeneratedClient(hooks, defaultUrlListServiceClient{ServiceClient: generatedruntime.NewServiceClient[*networkfirewallv1beta1.UrlList](buildUrlListGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*networkfirewallv1beta1.UrlList]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 2 * time.Second, Timeout: 10 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *networkfirewallv1beta1.UrlList) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *networkfirewallv1beta1.UrlList) error {
			if current.Status.Name != current.Spec.Name {
				return fmt.Errorf("created UrlList status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *networkfirewallv1beta1.UrlList) {
			current.Spec.Description = "OSOK recorded UrlList updated"
		},
		ValidateUpdated: func(current *networkfirewallv1beta1.UrlList) error {
			if current.Status.Description != "OSOK recorded UrlList updated" {
				return fmt.Errorf("updated UrlList status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredUrlListEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
