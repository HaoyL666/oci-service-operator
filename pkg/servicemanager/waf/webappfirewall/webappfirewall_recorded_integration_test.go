/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package webappfirewall

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	wafv1beta1 "github.com/oracle/oci-service-operator/api/waf/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

const recordedWebAppFirewallName = "osok-replay-waf-v1"

func TestRecordedWebAppFirewallCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "waf", Resource: "WebAppFirewall", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenWAFSDK(t, mode, filepath.Join("testdata", "recordings", "webappfirewall_crud.yaml"), metadata)
	compartmentID, policyID, loadBalancerID := "ocid1.compartment.oc1..replay", "ocid1.webappfirewallpolicy.oc1..replay", "ocid1.loadbalancer.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredWebAppFirewallRecordingEnv(t, "OCI_COMPARTMENT_ID")
		policyID = requiredWebAppFirewallRecordingEnv(t, "OCI_REPLAY_WAF_POLICY_ID")
		loadBalancerID = requiredWebAppFirewallRecordingEnv(t, "OCI_REPLAY_LOAD_BALANCER_ID")
	}
	resource := &wafv1beta1.WebAppFirewall{Spec: wafv1beta1.WebAppFirewallSpec{CompartmentId: compartmentID, WebAppFirewallPolicyId: policyID, BackendType: webAppFirewallBackendTypeLoadBalancer, LoadBalancerId: loadBalancerID, DisplayName: recordedWebAppFirewallName, FreeformTags: map[string]string{"osok-replay": "create"}}}
	client := newWebAppFirewallServiceClientWithOCIClient(loggerutil.OSOKLogger{}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*wafv1beta1.WebAppFirewall]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) }, HasIdentity: func(current *wafv1beta1.WebAppFirewall) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *wafv1beta1.WebAppFirewall) error {
			if current.Status.DisplayName != recordedWebAppFirewallName || current.Status.Id == "" {
				return fmt.Errorf("created WebAppFirewall status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *wafv1beta1.WebAppFirewall) {
			current.Spec.DisplayName = recordedWebAppFirewallName + "-updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *wafv1beta1.WebAppFirewall) error {
			if current.Status.DisplayName != current.Spec.DisplayName || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated WebAppFirewall status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredWebAppFirewallRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
