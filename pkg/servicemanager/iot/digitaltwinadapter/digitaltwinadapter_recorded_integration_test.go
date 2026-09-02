/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package digitaltwinadapter

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

func TestRecordedDigitalTwinAdapterCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "iot",
		Resource: "DigitalTwinAdapter",
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
	modelID := "ocid1.digitaltwinmodel.oc1..replay"
	if mode == ocireplay.ModeRecord {
		domainID = requiredDigitalTwinAdapterEnv(t, "OCI_REPLAY_IOT_DOMAIN_ID")
		modelID = requiredDigitalTwinAdapterEnv(t, "OCI_REPLAY_IOT_MODEL_ID")
	}
	sdkClient, closeSession := ocireplay.OpenIoTSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "digitaltwinadapter_crud.yaml"),
		metadata,
	)
	resource := &iotv1beta1.DigitalTwinAdapter{
		Spec: iotv1beta1.DigitalTwinAdapterSpec{
			IotDomainId:             domainID,
			DigitalTwinModelId:      modelID,
			DigitalTwinModelSpecUri: recordedDigitalTwinAdapterSpecURI,
			DisplayName:             "osok-replay-digital-twin-adapter",
			Description:             "OSOK recorded digital twin adapter",
			InboundEnvelope: iotv1beta1.DigitalTwinAdapterInboundEnvelope{
				ReferenceEndpoint: "device/temperature",
				ReferencePayload: iotv1beta1.DigitalTwinAdapterInboundEnvelopeReferencePayload{
					DataFormat: "JSON",
					Data: map[string]shared.JSONValue{
						"temperature": {Raw: []byte("72")},
						"time":        {Raw: []byte(`"2026-01-01T00:00:00Z"`)},
					},
				},
				EnvelopeMapping: iotv1beta1.DigitalTwinAdapterInboundEnvelopeEnvelopeMapping{
					TimeObserved: "$.time",
				},
			},
			InboundRoutes: []iotv1beta1.DigitalTwinAdapterInboundRoute{{
				Condition: "*",
				ReferencePayload: iotv1beta1.DigitalTwinAdapterInboundRouteReferencePayload{
					JsonData: `{"temperature":72}`,
				},
				PayloadMapping: map[string]string{"$.temperature": "$.temperature"},
				Description:    "default replay route",
			}},
			FreeformTags: map[string]string{"osok-replay": "create"},
		},
	}
	client := newDigitalTwinAdapterServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*iotv1beta1.DigitalTwinAdapter]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  10 * time.Second,
		Timeout:       20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *iotv1beta1.DigitalTwinAdapter) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *iotv1beta1.DigitalTwinAdapter) error {
			if current.Status.DisplayName != "osok-replay-digital-twin-adapter" || current.Status.LifecycleState != string(iotsdk.LifecycleStateActive) {
				return fmt.Errorf("created DigitalTwinAdapter status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *iotv1beta1.DigitalTwinAdapter) {
			current.Spec.Description = "OSOK recorded digital twin adapter updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *iotv1beta1.DigitalTwinAdapter) error {
			if current.Status.Description != "OSOK recorded digital twin adapter updated" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated DigitalTwinAdapter status = %+v", current.Status)
			}
			return nil
		},
	})
}

const recordedDigitalTwinAdapterSpecURI = "dtmi:com:oracle:osok:ReplayThermostat;1"

func requiredDigitalTwinAdapterEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
