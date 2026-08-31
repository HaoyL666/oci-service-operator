/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package topic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	onssdk "github.com/oracle/oci-go-sdk/v65/ons"
	onsv1beta1 "github.com/oracle/oci-service-operator/api/ons/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedTopicName = "osok-replay-common-topic-v1"
const recordedTopicPollInterval = 30 * time.Second

func TestRecordedTopicCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedTopicSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Topic cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredTopicRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &onsv1beta1.Topic{
		Spec: onsv1beta1.TopicSpec{
			Name:          recordedTopicName,
			CompartmentId: compartmentID,
			Description:   "recorded create",
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	client := newTopicServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)

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
			_ = ocireplay.Await(cleanupCtx, mode, recordedTopicPollInterval, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitTopicConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.OsokStatus.Reason != string(shared.Active) ||
		resource.Status.OsokStatus.Ocid == "" {
		t.Fatalf("created Topic status = %+v", resource.Status)
	}

	resource.Spec.Description = "recorded update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitTopicConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.OsokStatus.Reason != string(shared.Active) {
		t.Fatalf("updated Topic status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, recordedTopicPollInterval, func() (bool, error) {
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

func openRecordedTopicSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (onssdk.NotificationControlPlaneClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "ons",
		Resource: "Topic",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "topic_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := onssdk.NewNotificationControlPlaneClientWithConfigurationProvider(provider)
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
		Host:     "https://notification.us-ashburn-1.oraclecloud.com",
		BasePath: "20181201",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return onssdk.NotificationControlPlaneClient{
		BaseClient: session.BaseClient(),
	}, session.Close
}

func awaitTopicConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client TopicServiceClient,
	resource *onsv1beta1.Topic,
) error {
	return ocireplay.Await(ctx, mode, recordedTopicPollInterval, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Topic reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredTopicRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
