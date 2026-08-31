/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package queue

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	queuesdk "github.com/oracle/oci-go-sdk/v65/queue"
	queuev1beta1 "github.com/oracle/oci-service-operator/api/queue/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

const syntheticFailedQueueWorkRequestID = "ocid1.workrequest.oc1..syntheticfailed"

func TestSyntheticQueueFailedWorkRequestStopsReconciliation(t *testing.T) {
	sdkClient, closeSession := openSyntheticFailedQueueSDK(t)
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := closeSession(); err != nil {
				t.Errorf("close failed Queue work-request cassette: %v", err)
			}
		}
	})

	resource := &queuev1beta1.Queue{
		Spec: queuev1beta1.QueueSpec{
			DisplayName:   "osok-replay-synthetic-queue-failure-v1",
			CompartmentId: "ocid1.compartment.oc1..replay",
		},
	}
	resource.Status.CreateWorkRequestId = syntheticFailedQueueWorkRequestID
	manager := newQueueTestManager(sdkClient)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
	if err == nil {
		t.Fatal("Queue CreateOrUpdate() error = nil, want terminal work-request failure")
	}
	if response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("Queue CreateOrUpdate() response = %+v, want terminal failure", response)
	}
	if !strings.Contains(err.Error(), "work request "+syntheticFailedQueueWorkRequestID+" finished with status FAILED") {
		t.Fatalf("Queue CreateOrUpdate() error = %v", err)
	}
	if resource.Status.OsokStatus.Reason != string(shared.Failed) {
		t.Fatalf("Queue status.reason = %q, want %q", resource.Status.OsokStatus.Reason, shared.Failed)
	}
	current := resource.Status.OsokStatus.Async.Current
	if current == nil {
		t.Fatal("Queue status.async.current = nil, want failed work-request evidence")
	}
	if current.Source != shared.OSOKAsyncSourceWorkRequest ||
		current.Phase != shared.OSOKAsyncPhaseCreate ||
		current.WorkRequestID != syntheticFailedQueueWorkRequestID ||
		current.RawStatus != string(queuesdk.OperationStatusFailed) ||
		current.RawOperationType != string(queuesdk.OperationTypeCreateQueue) ||
		current.NormalizedClass != shared.OSOKAsyncClassFailed {
		t.Fatalf("Queue status.async.current = %+v", current)
	}

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func openSyntheticFailedQueueSDK(t *testing.T) (queuesdk.QueueAdminClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:    "queue",
		Resource:   "Queue",
		Operations: []ocireplay.Operation{ocireplay.OperationRead},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: filepath.Join(
			"testdata",
			"recordings",
			"queue_failed_work_request.yaml",
		),
		Host:     "https://messaging.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20210201",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return queuesdk.QueueAdminClient{BaseClient: session.BaseClient()}, session.Close
}
