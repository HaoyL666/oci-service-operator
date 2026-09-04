/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package webappaccelerationpolicy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	waasdk "github.com/oracle/oci-go-sdk/v65/waa"
	waav1beta1 "github.com/oracle/oci-service-operator/api/waa/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedWebAppAccelerationPolicyName = "osok-replay-waa-policy-v1"

func TestRecordedWebAppAccelerationPolicyCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	resource := makeWebAppAccelerationPolicyResource()
	resource.UID = "waa-policy-recorded-v1"
	resource.Spec.CompartmentId = "ocid1.compartment.oc1..replay"
	resource.Spec.DisplayName = recordedWebAppAccelerationPolicyName
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "create"}
	resource.Spec.DefinedTags = nil
	if mode == ocireplay.ModeRecord {
		resource.UID = k8stypes.UID(fmt.Sprintf("waa-policy-recorded-%d", time.Now().UnixNano()))
		resource.Spec.CompartmentId = requiredWebAppAccelerationPolicyEnv(t, "OCI_COMPARTMENT_ID")
	}

	sdkClient, closeSession := openRecordedWebAppAccelerationPolicySDK(t, mode)
	client := newWebAppAccelerationPolicyServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*waav1beta1.WebAppAccelerationPolicy]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 10 * time.Second, Timeout: 30 * time.Minute, CleanupTimeout: 30 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *waav1beta1.WebAppAccelerationPolicy) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *waav1beta1.WebAppAccelerationPolicy) error {
			if current.Status.Id == "" || current.Status.LifecycleState != string(waasdk.WebAppAccelerationPolicyLifecycleStateActive) {
				return fmt.Errorf("created WebAppAccelerationPolicy status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *waav1beta1.WebAppAccelerationPolicy) {
			current.Spec.DisplayName = recordedWebAppAccelerationPolicyName + "-updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *waav1beta1.WebAppAccelerationPolicy) error {
			if current.Status.DisplayName != current.Spec.DisplayName || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated WebAppAccelerationPolicy status = %+v", current.Status)
			}
			return nil
		},
		RetryError:       func(err error) bool { return ocireplay.IsHTTPStatus(err, 429) },
		RetryDeleteError: func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429) },
	})
}

type recordedWebAppAccelerationPolicyOCIClient struct {
	waasdk.WaaClient
	waasdk.WorkRequestClient
}

func openRecordedWebAppAccelerationPolicySDK(t *testing.T, mode ocireplay.Mode) (webAppAccelerationPolicyOCIClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "waa", Resource: "WebAppAccelerationPolicy",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "webappaccelerationpolicy_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := waasdk.NewWaaClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return recordedWebAppAccelerationPolicyOCIClient{WaaClient: client, WorkRequestClient: waasdk.WorkRequestClient{BaseClient: client.BaseClient}}, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://waa.us-ashburn-1.oci.oraclecloud.com", BasePath: "20211230", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	baseClient := session.BaseClient()
	return recordedWebAppAccelerationPolicyOCIClient{WaaClient: waasdk.WaaClient{BaseClient: baseClient}, WorkRequestClient: waasdk.WorkRequestClient{BaseClient: baseClient}}, session.Close
}

func requiredWebAppAccelerationPolicyEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
