/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package batchtaskprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	batchsdk "github.com/oracle/oci-go-sdk/v65/batch"
	batchv1beta1 "github.com/oracle/oci-service-operator/api/batch/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedBatchTaskProfileName = "osok-replay-batch-task-profile-v3"

func TestRecordedBatchTaskProfileCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedBatchTaskProfileSDK(t, mode)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredBatchTaskProfileRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &batchv1beta1.BatchTaskProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "recorded-batch-task-profile-v3"},
		Spec: batchv1beta1.BatchTaskProfileSpec{
			CompartmentId:  compartmentID,
			MinOcpus:       1,
			MinMemoryInGBs: 1,
			DisplayName:    recordedBatchTaskProfileName,
			Description:    "recorded create",
			FreeformTags:   map[string]string{"osok-replay": "create"},
		},
	}
	manager := &BatchTaskProfileServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newBatchTaskProfileRuntimeHooks(manager, sdkClient)
	client := wrapBatchTaskProfileGeneratedClient(hooks, defaultBatchTaskProfileServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*batchv1beta1.BatchTaskProfile](buildBatchTaskProfileGeneratedRuntimeConfig(manager, hooks)),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*batchv1beta1.BatchTaskProfile]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 2 * time.Second, Timeout: 10 * time.Minute, CleanupTimeout: 5 * time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		RetryError: func(err error) bool {
			return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429)
		},
		RetryDeleteError: func(err error) bool {
			return ocireplay.IsHTTPStatus(err, 409) || ocireplay.IsHTTPStatus(err, 429)
		},
		HasIdentity: func(current *batchv1beta1.BatchTaskProfile) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *batchv1beta1.BatchTaskProfile) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.DisplayName != recordedBatchTaskProfileName || current.Status.MinOcpus != 1 || current.Status.MinMemoryInGBs != 1 {
				return fmt.Errorf("created BatchTaskProfile status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *batchv1beta1.BatchTaskProfile) {
			current.Spec.Description = "recorded update"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *batchv1beta1.BatchTaskProfile) error {
			if current.Status.Description != "recorded update" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated BatchTaskProfile status = %+v", current.Status)
			}
			return nil
		},
	})
}

func openRecordedBatchTaskProfileSDK(t *testing.T, mode ocireplay.Mode) (batchsdk.BatchComputingClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "batch", Resource: "BatchTaskProfile",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "batchtaskprofile_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := batchsdk.NewBatchComputingClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://batch.us-ashburn-1.oci.oraclecloud.com", BasePath: "20251031", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return batchsdk.BatchComputingClient{BaseClient: session.BaseClient()}, session.Close
}

func requiredBatchTaskProfileRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
