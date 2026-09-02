/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package fleet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	jmsv1beta1 "github.com/oracle/oci-service-operator/api/jms/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedFleetCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "jms",
		Resource: "Fleet",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	logGroupID := "ocid1.loggroup.oc1..replay"
	logID := "ocid1.log.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredFleetRecordingEnv(t, "OCI_COMPARTMENT_ID")
		logGroupID = requiredFleetRecordingEnv(t, "OCI_REPLAY_JMS_LOG_GROUP_ID")
		logID = requiredFleetRecordingEnv(t, "OCI_REPLAY_JMS_LOG_ID")
	}
	sdkClient, closeSession := ocireplay.OpenJMSSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "fleet_crud.yaml"),
		metadata,
	)
	resource := &jmsv1beta1.Fleet{
		Spec: jmsv1beta1.FleetSpec{
			DisplayName:   "osok-replay-jms-fleet",
			CompartmentId: compartmentID,
			InventoryLog: jmsv1beta1.FleetInventoryLog{
				LogGroupId: logGroupID,
				LogId:      logID,
			},
			Description:  "OSOK recorded JMS fleet",
			FreeformTags: map[string]string{"osok-replay": "create"},
		},
	}
	client := newFleetServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*jmsv1beta1.Fleet]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  10 * time.Second,
		Timeout:       20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *jmsv1beta1.Fleet) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *jmsv1beta1.Fleet) error {
			if current.Status.DisplayName != "osok-replay-jms-fleet" {
				return fmt.Errorf("created Fleet status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *jmsv1beta1.Fleet) {
			current.Spec.Description = "OSOK recorded JMS fleet updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *jmsv1beta1.Fleet) error {
			if current.Status.Description != "OSOK recorded JMS fleet updated" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated Fleet status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredFleetRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
