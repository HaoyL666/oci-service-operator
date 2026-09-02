/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package protectionpolicy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	recoveryv1beta1 "github.com/oracle/oci-service-operator/api/recovery/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedProtectionPolicyCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "recovery", Resource: "ProtectionPolicy", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredProtectionPolicyEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := ocireplay.OpenRecoverySDK(t, mode, filepath.Join("testdata", "recordings", "protectionpolicy_crud.yaml"), metadata)
	resource := &recoveryv1beta1.ProtectionPolicy{Spec: recoveryv1beta1.ProtectionPolicySpec{DisplayName: "osok-replay-protection-policy", BackupRetentionPeriodInDays: 14, CompartmentId: compartmentID, FreeformTags: map[string]string{"osok-replay": "create"}}}
	client := newProtectionPolicyServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*recoveryv1beta1.ProtectionPolicy]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		HasIdentity: func(current *recoveryv1beta1.ProtectionPolicy) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *recoveryv1beta1.ProtectionPolicy) error {
			if current.Status.DisplayName != "osok-replay-protection-policy" || current.Status.BackupRetentionPeriodInDays != 14 {
				return fmt.Errorf("created ProtectionPolicy status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *recoveryv1beta1.ProtectionPolicy) {
			current.Spec.DisplayName = "osok-replay-protection-policy-updated"
			current.Spec.BackupRetentionPeriodInDays = 15
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *recoveryv1beta1.ProtectionPolicy) error {
			if current.Status.DisplayName != "osok-replay-protection-policy-updated" || current.Status.BackupRetentionPeriodInDays != 15 {
				return fmt.Errorf("updated ProtectionPolicy status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredProtectionPolicyEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
