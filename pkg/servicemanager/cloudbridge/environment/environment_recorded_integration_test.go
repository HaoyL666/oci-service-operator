/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package environment

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	cloudbridgesdk "github.com/oracle/oci-go-sdk/v65/cloudbridge"
	cloudbridgev1beta1 "github.com/oracle/oci-service-operator/api/cloudbridge/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedEnvironmentName = "osok-replay-cloudbridge-environment-v3"

func TestRecordedEnvironmentCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedEnvironmentSDK(t, mode)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredEnvironmentRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &cloudbridgev1beta1.Environment{ObjectMeta: metav1.ObjectMeta{Name: "recorded-environment"}, Spec: cloudbridgev1beta1.EnvironmentSpec{
		CompartmentId: compartmentID, DisplayName: recordedEnvironmentName,
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	manager := &EnvironmentServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newEnvironmentRuntimeHooks(manager, sdkClient)
	client := wrapEnvironmentGeneratedClient(hooks, defaultEnvironmentServiceClient{ServiceClient: generatedruntime.NewServiceClient[*cloudbridgev1beta1.Environment](buildEnvironmentGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudbridgev1beta1.Environment]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *cloudbridgev1beta1.Environment) bool { return current.Status.Id != "" },
		ValidateCreated: func(current *cloudbridgev1beta1.Environment) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedEnvironmentName {
				return fmt.Errorf("created Environment status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *cloudbridgev1beta1.Environment) {
			current.Spec.DisplayName = recordedEnvironmentName + "-updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *cloudbridgev1beta1.Environment) error {
			if current.Status.DisplayName != current.Spec.DisplayName || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated Environment status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedEnvironmentSDK(t *testing.T, mode ocireplay.Mode) (cloudbridgesdk.OcbAgentSvcClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "cloudbridge", Resource: "Environment", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "environment_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := cloudbridgesdk.NewOcbAgentSvcClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://cloudbridge.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220509", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return cloudbridgesdk.OcbAgentSvcClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredEnvironmentRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
