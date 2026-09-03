/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package softwaresource

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	osmanagementhubsdk "github.com/oracle/oci-go-sdk/v65/osmanagementhub"
	osmanagementhubv1beta1 "github.com/oracle/oci-service-operator/api/osmanagementhub/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

const recordedSoftwareSourceName = "osok-replay-private-source"

func TestRecordedSoftwareSourceCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredSoftwareSourceEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := openRecordedSoftwareSourceSDK(t, mode)
	resource := &osmanagementhubv1beta1.SoftwareSource{Spec: osmanagementhubv1beta1.SoftwareSourceSpec{
		CompartmentId: compartmentID, DisplayName: recordedSoftwareSourceName, Description: "recorded create",
		SoftwareSourceType: string(osmanagementhubsdk.SoftwareSourceTypePrivate), Url: "https://yum.oracle.com/repo/OracleLinux/OL8/baseos/latest/x86_64/",
		OsFamily: string(osmanagementhubsdk.OsFamilyOracleLinux8), ArchType: string(osmanagementhubsdk.ArchTypeX8664),
		IsGpgCheckEnabled: false, IsSslVerifyEnabled: true, FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	client := newSoftwareSourceServiceClientWithOCIClient(sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*osmanagementhubv1beta1.SoftwareSource]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 10 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		RetryError:    func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429) },
		HasIdentity: func(current *osmanagementhubv1beta1.SoftwareSource) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *osmanagementhubv1beta1.SoftwareSource) error {
			if current.Status.Id == "" || current.Status.DisplayName != recordedSoftwareSourceName {
				return fmt.Errorf("created SoftwareSource status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *osmanagementhubv1beta1.SoftwareSource) {
			current.Spec.Description = "recorded update"
		},
		ValidateUpdated: func(current *osmanagementhubv1beta1.SoftwareSource) error {
			if current.Status.Description != "recorded update" {
				return fmt.Errorf("updated SoftwareSource status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedSoftwareSourceSDK(t *testing.T, mode ocireplay.Mode) (osmanagementhubsdk.SoftwareSourceClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "osmanagementhub", Resource: "SoftwareSource", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "softwaresource_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := osmanagementhubsdk.NewSoftwareSourceClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://osmh.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220901", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return osmanagementhubsdk.SoftwareSourceClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredSoftwareSourceEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
