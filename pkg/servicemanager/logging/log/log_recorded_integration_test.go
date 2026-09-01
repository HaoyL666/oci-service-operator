/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package log

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
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedLogName = "osok-replay-custom-log-v1"

func TestRecordedLogCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedLogSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Log cassette: %v", err)
			}
		}
	})

	logGroupID := "ocid1.loggroup.oc1..replay"
	if mode == ocireplay.ModeRecord {
		logGroupID = requiredLogRecordingEnv(t, "OCI_REPLAY_LOG_GROUP_ID")
	}
	resource := &loggingv1beta1.Log{Spec: loggingv1beta1.LogSpec{
		DisplayName:       recordedLogName,
		LogType:           string(loggingsdk.CreateLogDetailsLogTypeCustom),
		IsEnabled:         true,
		FreeformTags:      map[string]string{"osok-replay": "create"},
		RetentionDuration: 30,
		LogGroupId:        logGroupID,
	}}
	client := newRecordedLogClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 10*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitLogConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(loggingsdk.LogLifecycleStateActive) ||
		resource.Status.DisplayName != recordedLogName ||
		!resource.Status.IsEnabled {
		t.Fatalf("created Log status = %+v", resource.Status)
	}

	resource.Spec.IsEnabled = false
	resource.Spec.RetentionDuration = 60
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitLogConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.IsEnabled || resource.Status.RetentionDuration != 60 ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Log status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	deleted = true

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func newRecordedLogClient(sdkClient loggingsdk.LoggingManagementClient) LogServiceClient {
	manager := &LogServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}
	hooks := newLogDefaultRuntimeHooks(sdkClient)
	applyLogRuntimeHooks(&hooks)
	delegate := defaultLogServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*loggingv1beta1.Log](
			buildLogGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return wrapLogGeneratedClient(hooks, delegate)
}

func openRecordedLogSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (loggingsdk.LoggingManagementClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "logging",
		Resource: "Log",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "log_crud.yaml")

	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := loggingsdk.NewLoggingManagementClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path:       cassettePath,
			Metadata:   metadata,
			BaseClient: &client.BaseClient,
			Overwrite:  ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}

	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path:     cassettePath,
		Host:     "https://logging.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20200531",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return loggingsdk.LoggingManagementClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitLogConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client LogServiceClient,
	resource *loggingv1beta1.Log,
) error {
	return ocireplay.Await(ctx, mode, 10*time.Second, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Log reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredLogRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
