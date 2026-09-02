/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package iotdomain

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

func TestRecordedIotDomainCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "iot",
		Resource: "IotDomain",
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
	domainGroupID := "ocid1.iotdomaingroup.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredIotDomainEnv(t, "OCI_COMPARTMENT_ID")
		domainGroupID = requiredIotDomainEnv(t, "OCI_REPLAY_IOT_DOMAIN_GROUP_ID")
	}
	sdkClient, closeSession := ocireplay.OpenIoTSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "iotdomain_crud.yaml"),
		metadata,
	)
	resource := &iotv1beta1.IotDomain{
		Spec: iotv1beta1.IotDomainSpec{
			IotDomainGroupId: domainGroupID,
			CompartmentId:    compartmentID,
			DisplayName:      "osok-replay-iot-domain",
			Description:      "OSOK recorded IoT domain",
			FreeformTags:     map[string]string{"osok-replay": "create"},
		},
	}
	client := newIotDomainServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*iotv1beta1.IotDomain]{
		Mode:           mode,
		Resource:       resource,
		Client:         client,
		CloseSession:   closeSession,
		PollInterval:   10 * time.Second,
		Timeout:        30 * time.Minute,
		CleanupTimeout: 20 * time.Minute,
		CreateContext:  func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *iotv1beta1.IotDomain) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *iotv1beta1.IotDomain) error {
			if current.Status.DisplayName != "osok-replay-iot-domain" || current.Status.LifecycleState != string(iotsdk.IotDomainLifecycleStateActive) {
				return fmt.Errorf("created IotDomain status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *iotv1beta1.IotDomain) {
			current.Spec.Description = "OSOK recorded IoT domain updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *iotv1beta1.IotDomain) error {
			if current.Status.Description != "OSOK recorded IoT domain updated" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated IotDomain status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredIotDomainEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
