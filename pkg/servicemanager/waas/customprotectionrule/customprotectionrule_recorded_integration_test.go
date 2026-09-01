/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package customprotectionrule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	waasv1beta1 "github.com/oracle/oci-service-operator/api/waas/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

const recordedCustomProtectionRuleName = "osok-replay-custom-protection-rule-v1"

func TestRecordedCustomProtectionRuleCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "waas", Resource: "CustomProtectionRule", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenWAASSDK(t, mode, filepath.Join("testdata", "recordings", "customprotectionrule_crud.yaml"), metadata)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredCustomProtectionRuleRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := makeCustomProtectionRuleResource()
	resource.Spec.CompartmentId = compartmentID
	resource.Spec.DisplayName = recordedCustomProtectionRuleName
	resource.Spec.DefinedTags = nil
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "create"}
	hooks := newCustomProtectionRuleDefaultRuntimeHooks(sdkClient)
	applyCustomProtectionRuleRuntimeHooks(&hooks)
	manager := &CustomProtectionRuleServiceManager{Log: loggerutil.OSOKLogger{}}
	client := wrapCustomProtectionRuleGeneratedClient(hooks, defaultCustomProtectionRuleServiceClient{ServiceClient: generatedruntime.NewServiceClient[*waasv1beta1.CustomProtectionRule](buildCustomProtectionRuleGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*waasv1beta1.CustomProtectionRule]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) }, HasIdentity: func(current *waasv1beta1.CustomProtectionRule) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *waasv1beta1.CustomProtectionRule) error {
			if current.Status.DisplayName != recordedCustomProtectionRuleName || current.Status.Id == "" {
				return fmt.Errorf("created CustomProtectionRule status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *waasv1beta1.CustomProtectionRule) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *waasv1beta1.CustomProtectionRule) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated CustomProtectionRule status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredCustomProtectionRuleRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
