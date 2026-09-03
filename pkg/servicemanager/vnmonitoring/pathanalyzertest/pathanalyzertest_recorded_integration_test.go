/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package pathanalyzertest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	vnmonitoringsdk "github.com/oracle/oci-go-sdk/v65/vnmonitoring"
	vnmonitoringv1beta1 "github.com/oracle/oci-service-operator/api/vnmonitoring/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

const recordedPathAnalyzerTestName = "osok-replay-path-analysis"

func TestRecordedPathAnalyzerTestCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredPathAnalyzerTestEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := openRecordedPathAnalyzerTestSDK(t, mode)
	resource := &vnmonitoringv1beta1.PathAnalyzerTest{Spec: vnmonitoringv1beta1.PathAnalyzerTestSpec{
		CompartmentId: compartmentID, DisplayName: recordedPathAnalyzerTestName, Protocol: 6,
		SourceEndpoint:      vnmonitoringv1beta1.PathAnalyzerTestSourceEndpoint{Type: "IP_ADDRESS", Address: "10.0.0.10"},
		DestinationEndpoint: vnmonitoringv1beta1.PathAnalyzerTestDestinationEndpoint{Type: "IP_ADDRESS", Address: "10.0.1.20"},
		ProtocolParameters:  vnmonitoringv1beta1.PathAnalyzerTestProtocolParameters{Type: "TCP", DestinationPort: 443},
		QueryOptions:        vnmonitoringv1beta1.PathAnalyzerTestQueryOptions{IsBiDirectionalAnalysis: true},
		FreeformTags:        map[string]string{"osok-replay": "create"},
	}}
	client := newPathAnalyzerTestServiceClientWithOCIClient(sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*vnmonitoringv1beta1.PathAnalyzerTest]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *vnmonitoringv1beta1.PathAnalyzerTest) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *vnmonitoringv1beta1.PathAnalyzerTest) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedPathAnalyzerTestName {
				return fmt.Errorf("created PathAnalyzerTest status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *vnmonitoringv1beta1.PathAnalyzerTest) {
			current.Spec.DisplayName = recordedPathAnalyzerTestName + "-updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *vnmonitoringv1beta1.PathAnalyzerTest) error {
			if current.Status.DisplayName != current.Spec.DisplayName || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated PathAnalyzerTest status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedPathAnalyzerTestSDK(t *testing.T, mode ocireplay.Mode) (vnmonitoringsdk.VnMonitoringClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "vnmonitoring", Resource: "PathAnalyzerTest", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "pathanalyzertest_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := vnmonitoringsdk.NewVnMonitoringClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://vnca-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20160918", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return vnmonitoringsdk.VnMonitoringClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredPathAnalyzerTestEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
