/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package webappfirewallpolicy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	wafv1beta1 "github.com/oracle/oci-service-operator/api/waf/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

const recordedWebAppFirewallPolicyName = "osok-replay-waf-policy-v1"

func TestRecordedWebAppFirewallPolicyCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "waf", Resource: "WebAppFirewallPolicy", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenWAFSDK(t, mode, filepath.Join("testdata", "recordings", "webappfirewallpolicy_crud.yaml"), metadata)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredWebAppFirewallPolicyRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &wafv1beta1.WebAppFirewallPolicy{Spec: wafv1beta1.WebAppFirewallPolicySpec{CompartmentId: compartmentID, DisplayName: recordedWebAppFirewallPolicyName, FreeformTags: map[string]string{"osok-replay": "create"}}}
	client := newWebAppFirewallPolicyServiceClientWithOCIClient(sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*wafv1beta1.WebAppFirewallPolicy]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) }, HasIdentity: func(current *wafv1beta1.WebAppFirewallPolicy) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *wafv1beta1.WebAppFirewallPolicy) error {
			if current.Status.DisplayName != recordedWebAppFirewallPolicyName || current.Status.Id == "" {
				return fmt.Errorf("created WebAppFirewallPolicy status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *wafv1beta1.WebAppFirewallPolicy) {
			current.Spec.DisplayName = recordedWebAppFirewallPolicyName + "-updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *wafv1beta1.WebAppFirewallPolicy) error {
			if current.Status.DisplayName != current.Spec.DisplayName || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated WebAppFirewallPolicy status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredWebAppFirewallPolicyRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
