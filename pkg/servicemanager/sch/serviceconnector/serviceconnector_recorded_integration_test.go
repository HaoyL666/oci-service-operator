/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package serviceconnector

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	schsdk "github.com/oracle/oci-go-sdk/v65/sch"
	schv1beta1 "github.com/oracle/oci-service-operator/api/sch/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedServiceConnectorName = "osok-replay-service-connector-recorded-v1"

func TestRecordedServiceConnectorCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	resource := newServiceConnectorResource()
	resource.UID = "service-connector-recorded-v1"
	resource.Spec.DisplayName = recordedServiceConnectorName
	resource.Spec.Description = "recorded create"
	resource.Spec.Source.LogSources[0].LogGroupId = "_Audit"
	resource.Spec.Source.LogSources[0].LogId = ""
	resource.Spec.Tasks[0].Condition = "logContent = 'OSOK_REPLAY_NEVER_MATCH'"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "create"}
	resource.Spec.DefinedTags = nil
	if mode == ocireplay.ModeRecord {
		resource.UID = k8stypes.UID(fmt.Sprintf("service-connector-recorded-%d", time.Now().UnixNano()))
		compartmentID := requiredServiceConnectorEnv(t, "OCI_COMPARTMENT_ID")
		resource.Spec.CompartmentId = compartmentID
		resource.Spec.Source.LogSources[0].CompartmentId = compartmentID
		resource.Spec.Target.TopicId = requiredServiceConnectorEnv(t, "OCI_REPLAY_ONS_TOPIC_ID")
	}

	sdkClient, closeSession := openRecordedServiceConnectorSDK(t, mode)
	client := newServiceConnectorServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*schv1beta1.ServiceConnector]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 20 * time.Minute, CleanupTimeout: 20 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		RetryError: func(err error) bool {
			return ocireplay.IsHTTPStatus(err, 429)
		},
		HasIdentity: func(current *schv1beta1.ServiceConnector) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *schv1beta1.ServiceConnector) error {
			if current.Status.Id == "" || current.Status.LifecycleState != string(schsdk.LifecycleStateActive) {
				return fmt.Errorf("created ServiceConnector status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *schv1beta1.ServiceConnector) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *schv1beta1.ServiceConnector) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated ServiceConnector status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedServiceConnectorSDK(t *testing.T, mode ocireplay.Mode) (schsdk.ServiceConnectorClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "sch", Resource: "ServiceConnector",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "serviceconnector_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := schsdk.NewServiceConnectorClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://service-connector-hub.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200909", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return schsdk.ServiceConnectorClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredServiceConnectorEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
