/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package managementagentinstallkey

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	managementagentsdk "github.com/oracle/oci-go-sdk/v65/managementagent"
	managementagentv1beta1 "github.com/oracle/oci-service-operator/api/managementagent/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedInstallKeyName = "osok-replay-install-key-v1"

func TestRecordedManagementAgentInstallKeyCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedInstallKeySDK(t, mode)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredInstallKeyRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &managementagentv1beta1.ManagementAgentInstallKey{Spec: managementagentv1beta1.ManagementAgentInstallKeySpec{
		CompartmentId: compartmentID, DisplayName: recordedInstallKeyName,
		IsUnlimited: true, IsKeyActive: true,
	}}
	client := newManagementAgentInstallKeyServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*managementagentv1beta1.ManagementAgentInstallKey]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *managementagentv1beta1.ManagementAgentInstallKey) bool { return current.Status.Id != "" },
		ValidateCreated: func(current *managementagentv1beta1.ManagementAgentInstallKey) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedInstallKeyName {
				return fmt.Errorf("created ManagementAgentInstallKey status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *managementagentv1beta1.ManagementAgentInstallKey) {
			current.Spec.DisplayName = recordedInstallKeyName + "-updated"
		},
		ValidateUpdated: func(current *managementagentv1beta1.ManagementAgentInstallKey) error {
			if current.Status.DisplayName != current.Spec.DisplayName {
				return fmt.Errorf("updated ManagementAgentInstallKey status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedInstallKeySDK(t *testing.T, mode ocireplay.Mode) (managementagentsdk.ManagementAgentClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "managementagent", Resource: "ManagementAgentInstallKey", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "managementagentinstallkey_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := managementagentsdk.NewManagementAgentClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://management-agent.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200202", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return managementagentsdk.ManagementAgentClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredInstallKeyRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
