/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package stream

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	streamingsdk "github.com/oracle/oci-go-sdk/v65/streaming"
	streamingv1beta1 "github.com/oracle/oci-service-operator/api/streaming/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedStreamName = "osok-replay-common-stream-v1"

func TestRecordedStreamCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedStreamSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Stream cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredStreamRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &streamingv1beta1.Stream{
		Spec: streamingv1beta1.StreamSpec{
			Name:             recordedStreamName,
			Partitions:       1,
			CompartmentId:    compartmentID,
			RetentionInHours: 24,
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	client := newRecordedStreamClient(sdkClient)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 5*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitStreamConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(streamingsdk.StreamLifecycleStateActive) ||
		resource.Status.Name != recordedStreamName {
		t.Fatalf("created Stream status = %+v", resource.Status)
	}

	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitStreamConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Stream status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	if err := awaitRecordedStreamAbsent(ctx, mode, sdkClient, resource.Status.Id); err != nil {
		t.Fatal(err)
	}
	deleted = true

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func newRecordedStreamClient(sdkClient streamingsdk.StreamAdminClient) StreamServiceClient {
	manager := &StreamServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}
	hooks := newStreamDefaultRuntimeHooks(sdkClient)
	return defaultStreamServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*streamingv1beta1.Stream](
			buildStreamGeneratedRuntimeConfig(manager, hooks),
		),
	}
}

func openRecordedStreamSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (streamingsdk.StreamAdminClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "streaming",
		Resource: "Stream",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "stream_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := streamingsdk.NewStreamAdminClientWithConfigurationProvider(provider)
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
		Host:     "https://streaming.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20180418",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return streamingsdk.StreamAdminClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitStreamConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client StreamServiceClient,
	resource *streamingv1beta1.Stream,
) error {
	return ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Stream reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func awaitRecordedStreamAbsent(
	ctx context.Context,
	mode ocireplay.Mode,
	sdkClient streamingsdk.StreamAdminClient,
	streamID string,
) error {
	return ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		response, err := sdkClient.GetStream(ctx, streamingsdk.GetStreamRequest{
			StreamId: common.String(streamID),
		})
		if err == nil {
			return response.Stream.LifecycleState == streamingsdk.StreamLifecycleStateDeleted, nil
		}
		serviceErr, ok := common.IsServiceError(err)
		if ok && serviceErr.GetHTTPStatusCode() == 404 {
			return true, nil
		}
		return false, err
	})
}

func requiredStreamRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
