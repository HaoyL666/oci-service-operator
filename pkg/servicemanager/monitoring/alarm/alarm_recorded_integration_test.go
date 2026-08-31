/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package alarm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	monitoringsdk "github.com/oracle/oci-go-sdk/v65/monitoring"
	monitoringv1beta1 "github.com/oracle/oci-service-operator/api/monitoring/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedAlarmName = "osok-replay-common-alarm-v1"

func TestRecordedAlarmCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedAlarmSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Alarm cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	notificationTopicID := "ocid1.onstopic.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredAlarmRecordingEnv(t, "OCI_COMPARTMENT_ID")
		notificationTopicID = requiredAlarmRecordingEnv(t, "OCI_NOTIFICATION_TOPIC_ID")
	}
	resource := &monitoringv1beta1.Alarm{
		Spec: monitoringv1beta1.AlarmSpec{
			DisplayName:         recordedAlarmName,
			CompartmentId:       compartmentID,
			MetricCompartmentId: compartmentID,
			Namespace:           "oci_computeagent",
			Query:               "CpuUtilization[1m].mean() > 100",
			Severity:            "CRITICAL",
			Destinations:        []string{notificationTopicID},
			IsEnabled:           false,
			Body:                "Synthetic-safe recorded alarm; disabled during lifecycle validation.",
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	client := newRecordedAlarmClient(sdkClient)

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
	if err := awaitAlarmConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(monitoringsdk.AlarmLifecycleStateActive) ||
		resource.Status.DisplayName != recordedAlarmName ||
		resource.Status.IsEnabled {
		t.Fatalf("created Alarm status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedAlarmName + "-updated"
	resource.Spec.Query = "CpuUtilization[1m].mean() > 99"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitAlarmConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.Query != resource.Spec.Query ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Alarm status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
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

func newRecordedAlarmClient(sdkClient monitoringsdk.MonitoringClient) AlarmServiceClient {
	manager := &AlarmServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
	}
	hooks := newAlarmDefaultRuntimeHooks(sdkClient)
	return defaultAlarmServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*monitoringv1beta1.Alarm](
			buildAlarmGeneratedRuntimeConfig(manager, hooks),
		),
	}
}

func openRecordedAlarmSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (monitoringsdk.MonitoringClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "monitoring",
		Resource: "Alarm",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "alarm_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := monitoringsdk.NewMonitoringClientWithConfigurationProvider(provider)
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
		Host:     "https://telemetry.us-ashburn-1.oraclecloud.com",
		BasePath: "20180401",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return monitoringsdk.MonitoringClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitAlarmConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client AlarmServiceClient,
	resource *monitoringv1beta1.Alarm,
) error {
	return ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Alarm reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredAlarmRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
