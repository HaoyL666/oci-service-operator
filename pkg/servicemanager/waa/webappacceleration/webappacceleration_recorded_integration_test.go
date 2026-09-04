/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package webappacceleration

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

const recordedWebAppAccelerationName = "osok-replay-waa-acceleration-v1"

func TestRecordedWebAppAccelerationCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	resource := makeWebAppAccelerationResource()
	resource.UID = "waa-acceleration-recorded-v1"
	resource.Spec.CompartmentId = "ocid1.compartment.oc1..replay"
	resource.Spec.WebAppAccelerationPolicyId = "ocid1.webappaccelerationpolicy.oc1..replay"
	resource.Spec.LoadBalancerId = "ocid1.loadbalancer.oc1..replay"
	resource.Spec.DisplayName = recordedWebAppAccelerationName
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "create"}
	resource.Spec.DefinedTags = nil
	resource.Spec.SystemTags = nil
	if mode == ocireplay.ModeRecord {
		resource.UID = k8stypes.UID(fmt.Sprintf("waa-acceleration-recorded-%d", time.Now().UnixNano()))
		resource.Spec.CompartmentId = requiredWebAppAccelerationEnv(t, "OCI_COMPARTMENT_ID")
		resource.Spec.WebAppAccelerationPolicyId = requiredWebAppAccelerationEnv(t, "OCI_REPLAY_WAA_POLICY_ID")
		resource.Spec.LoadBalancerId = requiredWebAppAccelerationEnv(t, "OCI_REPLAY_LOAD_BALANCER_ID")
	}

	sdkClient, closeSession := openRecordedWebAppAccelerationSDK(t, mode)
	client := newWebAppAccelerationServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*waav1beta1.WebAppAcceleration]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 10 * time.Second, Timeout: 30 * time.Minute, CleanupTimeout: 30 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *waav1beta1.WebAppAcceleration) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *waav1beta1.WebAppAcceleration) error {
			if current.Status.Id == "" || current.Status.LifecycleState != string(waasdk.WebAppAccelerationLifecycleStateActive) {
				return fmt.Errorf("created WebAppAcceleration status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *waav1beta1.WebAppAcceleration) {
			current.Spec.DisplayName = recordedWebAppAccelerationName + "-updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *waav1beta1.WebAppAcceleration) error {
			if current.Status.DisplayName != current.Spec.DisplayName || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated WebAppAcceleration status = %+v", current.Status)
			}
			return nil
		},
		RetryError:       func(err error) bool { return ocireplay.IsHTTPStatus(err, 429) },
		RetryDeleteError: func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429) },
	})
}

type recordedWebAppAccelerationOCIClient struct {
	waasdk.WaaClient
	waasdk.WorkRequestClient
}

func openRecordedWebAppAccelerationSDK(t *testing.T, mode ocireplay.Mode) (webAppAccelerationOCIClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "waa", Resource: "WebAppAcceleration",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "webappacceleration_crud.yaml")
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
		return recordedWebAppAccelerationOCIClient{WaaClient: client, WorkRequestClient: waasdk.WorkRequestClient{BaseClient: client.BaseClient}}, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://waa.us-ashburn-1.oci.oraclecloud.com", BasePath: "20211230", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	baseClient := session.BaseClient()
	return recordedWebAppAccelerationOCIClient{WaaClient: waasdk.WaaClient{BaseClient: baseClient}, WorkRequestClient: waasdk.WorkRequestClient{BaseClient: baseClient}}, session.Close
}

func requiredWebAppAccelerationEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
