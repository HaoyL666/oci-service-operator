/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package pingmonitor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	healthcheckssdk "github.com/oracle/oci-go-sdk/v65/healthchecks"
	healthchecksv1beta1 "github.com/oracle/oci-service-operator/api/healthchecks/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedPingMonitorName = "osok-replay-ping-monitor-v1"

func TestRecordedPingMonitorCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedPingMonitorSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close PingMonitor cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredPingMonitorRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &healthchecksv1beta1.PingMonitor{Spec: healthchecksv1beta1.PingMonitorSpec{
		CompartmentId:     compartmentID,
		Targets:           []string{"example.com"},
		Protocol:          string(healthcheckssdk.PingProbeProtocolTcp),
		DisplayName:       recordedPingMonitorName,
		IntervalInSeconds: 60,
		Port:              443,
		TimeoutInSeconds:  10,
		IsEnabled:         false,
		FreeformTags:      map[string]string{"osok-replay": "create"},
	}}
	client := newTestPingMonitorClient(sdkClient)
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
			_ = ocireplay.Await(cleanupCtx, mode, 5*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	if err := awaitRecordedPingMonitor(generatedruntime.WithSkipExistingBeforeCreate(ctx), mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != recordedPingMonitorName || resource.Status.IsEnabled {
		t.Fatalf("created PingMonitor status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedPingMonitorName + "-updated"
	resource.Spec.IntervalInSeconds = 30
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitRecordedPingMonitor(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.IntervalInSeconds != resource.Spec.IntervalInSeconds {
		t.Fatalf("updated PingMonitor status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, recordedPingMonitorPollInterval(mode), func() (bool, error) {
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

func openRecordedPingMonitorSDK(t *testing.T, mode ocireplay.Mode) (healthcheckssdk.HealthChecksClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "healthchecks", Resource: "PingMonitor", Operations: []ocireplay.Operation{
		ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete,
	}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "pingmonitor_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := healthcheckssdk.NewHealthChecksClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Overwrite: ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: path, Host: "https://healthchecks.us-ashburn-1.oci.oraclecloud.com", BasePath: "20180501", Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return healthcheckssdk.HealthChecksClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitRecordedPingMonitor(ctx context.Context, mode ocireplay.Mode, client PingMonitorServiceClient, resource *healthchecksv1beta1.PingMonitor) error {
	return ocireplay.Await(ctx, mode, recordedPingMonitorPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("PingMonitor reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func recordedPingMonitorPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 5 * time.Second
	}
	return 0
}

func requiredPingMonitorRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
