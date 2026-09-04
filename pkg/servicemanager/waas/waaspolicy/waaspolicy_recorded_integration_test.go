/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package waaspolicy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	waassdk "github.com/oracle/oci-go-sdk/v65/waas"
	waasv1beta1 "github.com/oracle/oci-service-operator/api/waas/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

const (
	recordedWaasPolicyName   = "osok-replay-waas-policy-recorded-v2"
	recordedWaasPolicyDomain = "osok-replay-waas-recorded-v2.example.com"
)

func TestRecordedWaasPolicyCreateReadDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	resource := newTestWaasPolicy()
	resource.UID = "waas-policy-recorded-v1"
	resource.Spec.CompartmentId = "ocid1.compartment.oc1..replay"
	resource.Spec.DisplayName = recordedWaasPolicyName
	resource.Spec.Domain = recordedWaasPolicyDomain
	resource.Spec.Origins["primary"] = waasv1beta1.WaasPolicyOrigins{Uri: "192.0.2.10"}
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "create"}
	if mode == ocireplay.ModeRecord {
		resource.UID = k8stypes.UID(fmt.Sprintf("waas-policy-recorded-%d", time.Now().UnixNano()))
		resource.Spec.CompartmentId = requiredWaasPolicyRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}

	sdkClient, closeSession := openRecordedWaasPolicySDK(t, mode)
	client := newWaasPolicyServiceClientWithOCIClient(sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*waasv1beta1.WaasPolicy]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 30 * time.Second, Timeout: 90 * time.Minute, CleanupTimeout: 90 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *waasv1beta1.WaasPolicy) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *waasv1beta1.WaasPolicy) error {
			if current.Status.Id == "" || current.Status.LifecycleState != string(waassdk.LifecycleStatesActive) {
				return fmt.Errorf("created WaasPolicy status = %+v", current.Status)
			}
			return nil
		},
		RetryError:       func(err error) bool { return ocireplay.IsHTTPStatus(err, 429) },
		RetryDeleteError: func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429) },
	})
}

func openRecordedWaasPolicySDK(t *testing.T, mode ocireplay.Mode) (waassdk.WaasClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "waas", Resource: "WaasPolicy",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "waaspolicy_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := waassdk.NewWaasClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://waas.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181116", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return waassdk.WaasClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredWaasPolicyRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
