/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package iotdomaingroup

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
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedIotDomainGroupCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "iot",
		Resource: "IotDomainGroup",
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
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredIotDomainGroupEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := ocireplay.OpenIoTSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "iotdomaingroup_crud.yaml"),
		metadata,
	)
	resource := &iotv1beta1.IotDomainGroup{
		Spec: iotv1beta1.IotDomainGroupSpec{
			CompartmentId: compartmentID,
			Type:          string(iotsdk.IotDomainGroupTypeLightweight),
			DisplayName:   "osok-replay-iot-domain-group",
			Description:   "OSOK recorded IoT domain group",
			FreeformTags:  map[string]string{"osok-replay": "create"},
		},
	}
	client := newIotDomainGroupServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*iotv1beta1.IotDomainGroup]{
		Mode:           mode,
		Resource:       resource,
		Client:         client,
		CloseSession:   closeSession,
		PollInterval:   10 * time.Second,
		Timeout:        30 * time.Minute,
		CleanupTimeout: 20 * time.Minute,
		CreateContext:  func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *iotv1beta1.IotDomainGroup) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *iotv1beta1.IotDomainGroup) error {
			if current.Status.DisplayName != "osok-replay-iot-domain-group" || current.Status.LifecycleState != string(iotsdk.IotDomainGroupLifecycleStateActive) {
				return fmt.Errorf("created IotDomainGroup status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *iotv1beta1.IotDomainGroup) {
			current.Spec.Description = "OSOK recorded IoT domain group updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *iotv1beta1.IotDomainGroup) error {
			if current.Status.Description != "OSOK recorded IoT domain group updated" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated IotDomainGroup status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredIotDomainGroupEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
