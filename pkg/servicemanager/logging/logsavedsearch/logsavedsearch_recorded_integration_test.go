/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package logsavedsearch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	loggingsdk "github.com/oracle/oci-go-sdk/v65/logging"
	loggingv1beta1 "github.com/oracle/oci-service-operator/api/logging/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedLogSavedSearchName = "osok-replay-log-saved-search-v1"

func TestRecordedLogSavedSearchCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedLogSavedSearchSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close LogSavedSearch cassette: %v", err)
			}
		}
	})
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredLogSavedSearchRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &loggingv1beta1.LogSavedSearch{Spec: loggingv1beta1.LogSavedSearchSpec{
		CompartmentId: compartmentID, Name: recordedLogSavedSearchName,
		Query: "search \"recorded-replay-token\" | sort by datetime desc", Description: "recorded create",
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	client := newTestLogSavedSearchClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.Id == "" {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 5*time.Second, func() (bool, error) { return client.Delete(cleanupCtx, resource) })
		})
	}
	if err := awaitRecordedLogSavedSearch(generatedruntime.WithSkipExistingBeforeCreate(ctx), mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Name != resource.Spec.Name || resource.Status.Id == "" {
		t.Fatalf("created LogSavedSearch status = %+v", resource.Status)
	}
	resource.Spec.Name = recordedLogSavedSearchName + "-updated"
	resource.Spec.Description = "recorded update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitRecordedLogSavedSearch(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Name != resource.Spec.Name || resource.Status.Description != resource.Spec.Description {
		t.Fatalf("updated LogSavedSearch status = %+v", resource.Status)
	}
	if err := ocireplay.Await(ctx, mode, recordedLogSavedSearchPollInterval(mode), func() (bool, error) { return client.Delete(ctx, resource) }); err != nil {
		t.Fatal(err)
	}
	deleted = true
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func openRecordedLogSavedSearchSDK(t *testing.T, mode ocireplay.Mode) (loggingsdk.LoggingManagementClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "logging", Resource: "LogSavedSearch", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "logsavedsearch_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := loggingsdk.NewLoggingManagementClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://logging.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200531", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	return loggingsdk.LoggingManagementClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitRecordedLogSavedSearch(ctx context.Context, mode ocireplay.Mode, client LogSavedSearchServiceClient, resource *loggingv1beta1.LogSavedSearch) error {
	return ocireplay.Await(ctx, mode, recordedLogSavedSearchPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("LogSavedSearch reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}
func recordedLogSavedSearchPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 5 * time.Second
	}
	return 0
}
func requiredLogSavedSearchRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
