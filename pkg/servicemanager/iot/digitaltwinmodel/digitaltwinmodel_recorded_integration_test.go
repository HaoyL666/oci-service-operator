/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package digitaltwinmodel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	iotsdk "github.com/oracle/oci-go-sdk/v65/iot"
	iotv1beta1 "github.com/oracle/oci-service-operator/api/iot/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedDigitalTwinModelSpecURI = "dtmi:com:oracle:osok:ReplayLifecycleThermostat;1"

func TestRecordedDigitalTwinModelCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "iot",
		Resource: "DigitalTwinModel",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	domainID := "ocid1.iotdomain.oc1..replay"
	if mode == ocireplay.ModeRecord {
		domainID = requiredDigitalTwinModelEnv(t, "OCI_REPLAY_IOT_DOMAIN_ID")
	}
	sdkClient, closeSession := ocireplay.OpenIoTSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "digitaltwinmodel_crud.yaml"),
		metadata,
	)
	resource := &iotv1beta1.DigitalTwinModel{
		Spec: iotv1beta1.DigitalTwinModelSpec{
			IotDomainId: domainID,
			Spec: map[string]shared.JSONValue{
				"@context":    {Raw: []byte(`"dtmi:dtdl:context;3"`)},
				"@id":         {Raw: []byte(`"` + recordedDigitalTwinModelSpecURI + `"`)},
				"@type":       {Raw: []byte(`"Interface"`)},
				"displayName": {Raw: []byte(`"OSOK Replay Thermostat"`)},
				"contents":    {Raw: []byte(`[{"@type":"Property","name":"temperature","schema":"double"}]`)},
			},
			DisplayName:  "osok-replay-digital-twin-model",
			Description:  "OSOK recorded digital twin model",
			FreeformTags: map[string]string{"osok-replay": "create"},
		},
	}
	client := newDigitalTwinModelServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*iotv1beta1.DigitalTwinModel]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  10 * time.Second,
		Timeout:       20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *iotv1beta1.DigitalTwinModel) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *iotv1beta1.DigitalTwinModel) error {
			if current.Status.DisplayName != "osok-replay-digital-twin-model" || current.Status.LifecycleState != string(iotsdk.LifecycleStateActive) {
				return fmt.Errorf("created DigitalTwinModel status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *iotv1beta1.DigitalTwinModel) {
			current.Spec.Description = "OSOK recorded digital twin model updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *iotv1beta1.DigitalTwinModel) error {
			if current.Status.Description != "OSOK recorded digital twin model updated" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated DigitalTwinModel status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredDigitalTwinModelEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
