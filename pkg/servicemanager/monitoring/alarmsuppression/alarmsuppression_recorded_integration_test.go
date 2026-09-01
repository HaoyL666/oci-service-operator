/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package alarmsuppression

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	monitoringv1beta1 "github.com/oracle/oci-service-operator/api/monitoring/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

const recordedAlarmSuppressionName = "osok-replay-alarm-suppression-v1"

func TestRecordedAlarmSuppressionCreateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "monitoring", Resource: "AlarmSuppression", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	sdkClient, closeSession := ocireplay.OpenMonitoringSDK(t, mode, filepath.Join("testdata", "recordings", "alarmsuppression_create_delete.yaml"), metadata)
	alarmID := "ocid1.alarm.oc1..replay"
	if mode == ocireplay.ModeRecord {
		alarmID = requiredAlarmSuppressionRecordingEnv(t, "OCI_REPLAY_ALARM_ID")
	}
	resource := &monitoringv1beta1.AlarmSuppression{Spec: monitoringv1beta1.AlarmSuppressionSpec{
		AlarmSuppressionTarget: monitoringv1beta1.AlarmSuppressionTarget{TargetType: "ALARM", AlarmId: alarmID}, DisplayName: recordedAlarmSuppressionName,
		TimeSuppressFrom: "2026-09-02T00:00:00Z", TimeSuppressUntil: "2026-09-02T01:00:00Z", Level: "ALARM", Description: "recorded maintenance",
	}}
	hooks := newAlarmSuppressionRuntimeHooks(&AlarmSuppressionServiceManager{Log: loggerutil.OSOKLogger{}}, sdkClient)
	manager := &AlarmSuppressionServiceManager{Log: loggerutil.OSOKLogger{}}
	client := wrapAlarmSuppressionGeneratedClient(hooks, defaultAlarmSuppressionServiceClient{ServiceClient: generatedruntime.NewServiceClient[*monitoringv1beta1.AlarmSuppression](buildAlarmSuppressionGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*monitoringv1beta1.AlarmSuppression]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *monitoringv1beta1.AlarmSuppression) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *monitoringv1beta1.AlarmSuppression) error {
			if current.Status.DisplayName != recordedAlarmSuppressionName || current.Status.Id == "" {
				return fmt.Errorf("created AlarmSuppression status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredAlarmSuppressionRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
