/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package suppression

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	emailsdk "github.com/oracle/oci-go-sdk/v65/email"
	emailv1beta1 "github.com/oracle/oci-service-operator/api/email/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedSuppressionAddress = "osok-replay-suppression@example.com"

func TestRecordedSuppressionCreateReadDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	tenancyID := "ocid1.tenancy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		tenancyID = requiredSuppressionRecordingEnv(t, "OCI_REPLAY_TENANCY_ID")
	}
	resource := &emailv1beta1.Suppression{Spec: emailv1beta1.SuppressionSpec{
		CompartmentId: tenancyID,
		EmailAddress:  recordedSuppressionAddress,
	}}
	sdkClient, closeSession := openRecordedSuppressionSDK(t, mode)
	manager := &SuppressionServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newSuppressionRuntimeHooks(manager, sdkClient)
	client := wrapSuppressionGeneratedClient(hooks, defaultSuppressionServiceClient{ServiceClient: generatedruntime.NewServiceClient[*emailv1beta1.Suppression](buildSuppressionGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*emailv1beta1.Suppression]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 10 * time.Minute, CleanupTimeout: 10 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *emailv1beta1.Suppression) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *emailv1beta1.Suppression) error {
			if current.Status.Id == "" || current.Status.EmailAddress != recordedSuppressionAddress {
				return fmt.Errorf("created Suppression status = %+v", current.Status)
			}
			return nil
		},
		RetryError:       func(err error) bool { return ocireplay.IsHTTPStatus(err, 429) },
		RetryDeleteError: func(err error) bool { return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429) },
	})
}

func openRecordedSuppressionSDK(t *testing.T, mode ocireplay.Mode) (emailsdk.EmailClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "email", Resource: "Suppression",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "suppression_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := emailsdk.NewEmailClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://ctrl.email.us-ashburn-1.oci.oraclecloud.com", BasePath: "20170907", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return emailsdk.EmailClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredSuppressionRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
