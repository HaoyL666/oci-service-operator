/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package drprotectiongroup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	disasterrecoverysdk "github.com/oracle/oci-go-sdk/v65/disasterrecovery"
	disasterrecoveryv1beta1 "github.com/oracle/oci-service-operator/api/disasterrecovery/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedDrProtectionGroupName = "osok-replay-dr-protection-group-v1"

func TestRecordedDrProtectionGroupCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedDrProtectionGroupSDK(t, mode)
	resource := newDrProtectionGroupTestResource()
	resource.Spec.CompartmentId = "ocid1.compartment.oc1..replay"
	resource.Spec.DisplayName = recordedDrProtectionGroupName
	resource.Spec.LogLocation.Namespace = "replaynamespace"
	resource.Spec.LogLocation.Bucket = "replay-dr-logs"
	resource.Spec.DefinedTags = nil
	resource.Spec.FreeformTags = map[string]string{"managed-by": "osok-replay"}
	if mode == ocireplay.ModeRecord {
		resource.Spec.CompartmentId = requiredDrProtectionGroupRecordingEnv(t, "OCI_COMPARTMENT_ID")
		resource.Spec.LogLocation.Namespace = requiredDrProtectionGroupRecordingEnv(t, "OCI_OBJECTSTORAGE_NAMESPACE")
		resource.Spec.LogLocation.Bucket = requiredDrProtectionGroupRecordingEnv(t, "OCI_DR_LOG_BUCKET_NAME")
	}
	hooks := newDrProtectionGroupDefaultRuntimeHooks(sdkClient)
	applyDrProtectionGroupRuntimeHooks(&hooks, sdkClient, nil)
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}
	manager := &DrProtectionGroupServiceManager{Log: log}
	client := wrapDrProtectionGroupGeneratedClient(hooks, defaultDrProtectionGroupServiceClient{ServiceClient: generatedruntime.NewServiceClient[*disasterrecoveryv1beta1.DrProtectionGroup](buildDrProtectionGroupGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*disasterrecoveryv1beta1.DrProtectionGroup]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 15 * time.Minute, CleanupTimeout: 10 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *disasterrecoveryv1beta1.DrProtectionGroup) bool {
			return current.Status.Id != "" || current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *disasterrecoveryv1beta1.DrProtectionGroup) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedDrProtectionGroupName {
				return fmt.Errorf("created DrProtectionGroup status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *disasterrecoveryv1beta1.DrProtectionGroup) {
			current.Spec.DisplayName = recordedDrProtectionGroupName + "-updated"
		},
		ValidateUpdated: func(current *disasterrecoveryv1beta1.DrProtectionGroup) error {
			if current.Status.DisplayName != current.Spec.DisplayName {
				return fmt.Errorf("updated DrProtectionGroup status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedDrProtectionGroupSDK(t *testing.T, mode ocireplay.Mode) (disasterrecoverysdk.DisasterRecoveryClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "disasterrecovery", Resource: "DrProtectionGroup", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "drprotectiongroup_crud.yaml")
	bindings := map[string]string{"objectstorage-namespace": "replaynamespace", "dr-log-bucket": "replay-dr-logs"}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := disasterrecoverysdk.NewDisasterRecoveryClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		bindings["objectstorage-namespace"] = requiredDrProtectionGroupRecordingEnv(t, "OCI_OBJECTSTORAGE_NAMESPACE")
		bindings["dr-log-bucket"] = requiredDrProtectionGroupRecordingEnv(t, "OCI_DR_LOG_BUCKET_NAME")
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://disaster-recovery.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220125", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return disasterrecoverysdk.DisasterRecoveryClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredDrProtectionGroupRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
