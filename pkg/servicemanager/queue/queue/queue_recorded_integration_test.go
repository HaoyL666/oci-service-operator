/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package queue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	queuesdk "github.com/oracle/oci-go-sdk/v65/queue"
	queuev1beta1 "github.com/oracle/oci-service-operator/api/queue/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedQueueName = "osok-replay-async-queue-v1"

func TestRecordedQueueCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedQueueSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Queue cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredQueueRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}

	resource := &queuev1beta1.Queue{
		Spec: queuev1beta1.QueueSpec{
			DisplayName:             recordedQueueName,
			CompartmentId:           compartmentID,
			RetentionInSeconds:      86400,
			VisibilityInSeconds:     30,
			TimeoutInSeconds:        20,
			ChannelConsumptionLimit: 100,
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	manager := newQueueTestManager(sdkClient)

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
				return manager.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitQueueConvergence(createCtx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(queuesdk.QueueLifecycleStateActive) ||
		resource.Status.DisplayName != recordedQueueName {
		t.Fatalf("created Queue status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedQueueName + "-updated"
	resource.Spec.VisibilityInSeconds = 45
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitQueueConvergence(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.VisibilityInSeconds != resource.Spec.VisibilityInSeconds ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Queue status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		return manager.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	deleted = true

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func openRecordedQueueSDK(t *testing.T, mode ocireplay.Mode) (queuesdk.QueueAdminClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "queue",
		Resource: "Queue",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "queue_crud.yaml")

	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := queuesdk.NewQueueAdminClientWithConfigurationProvider(provider)
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
		Host:     "https://messaging.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20210201",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return queuesdk.QueueAdminClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitQueueConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	manager *QueueServiceManager,
	resource *queuev1beta1.Queue,
) error {
	return ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Queue reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredQueueRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
