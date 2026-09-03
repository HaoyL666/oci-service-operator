/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package securitypolicyconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	datasafesdk "github.com/oracle/oci-go-sdk/v65/datasafe"
	datasafev1beta1 "github.com/oracle/oci-service-operator/api/datasafe/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedSecurityPolicyConfigName = "osok-replay-security-policy-config-v1"

func TestRecordedSecurityPolicyConfigCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	securityPolicyID := "ocid1.datasafesecuritypolicy.oc1.iad.replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredSecurityPolicyConfigRecordingEnv(t, "OCI_COMPARTMENT_ID")
		securityPolicyID = requiredSecurityPolicyConfigRecordingEnv(t, "OCI_REPLAY_SECURITY_POLICY_ID")
	}
	sdkClient, closeSession := openRecordedSecurityPolicyConfigSDK(t, mode, securityPolicyID)
	resource := &datasafev1beta1.SecurityPolicyConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "recorded-security-policy-config-v1"},
		Spec: datasafev1beta1.SecurityPolicyConfigSpec{
			CompartmentId:    compartmentID,
			SecurityPolicyId: securityPolicyID,
			DisplayName:      recordedSecurityPolicyConfigName,
			Description:      "recorded create",
			FirewallConfig: datasafev1beta1.SecurityPolicyConfigFirewallConfig{
				Status:                string(datasafesdk.FirewallConfigStatusEnabled),
				ViolationLogAutoPurge: string(datasafesdk.FirewallConfigViolationLogAutoPurgeDisabled),
				ExcludeJob:            string(datasafesdk.FirewallConfigExcludeJobIncluded),
			},
			UnifiedAuditPolicyConfig: datasafev1beta1.SecurityPolicyConfigUnifiedAuditPolicyConfig{
				ExcludeDatasafeUser: string(datasafesdk.UnifiedAuditPolicyConfigExcludeDatasafeUserEnabled),
			},
			FreeformTags: map[string]string{"osok-replay": "create"},
		},
	}
	client := newSecurityPolicyConfigServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.SecurityPolicyConfig]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 20 * time.Minute, CleanupTimeout: 10 * time.Minute,
		RetryError: func(err error) bool {
			return ocireplay.IsHTTPStatus(err, 404) || ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429)
		},
		HasIdentity: func(current *datasafev1beta1.SecurityPolicyConfig) bool {
			return current.Status.Id != "" || current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *datasafev1beta1.SecurityPolicyConfig) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedSecurityPolicyConfigName || current.Status.SecurityPolicyId != securityPolicyID {
				return fmt.Errorf("created SecurityPolicyConfig status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *datasafev1beta1.SecurityPolicyConfig) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *datasafev1beta1.SecurityPolicyConfig) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated SecurityPolicyConfig status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedSecurityPolicyConfigSDK(t *testing.T, mode ocireplay.Mode, securityPolicyID string) (datasafesdk.DataSafeClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "datasafe", Resource: "SecurityPolicyConfig",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "securitypolicyconfig_crud.yaml")
	bindings := map[string]string{"security-policy-id": securityPolicyID}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := datasafesdk.NewDataSafeClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredSecurityPolicyConfigRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
