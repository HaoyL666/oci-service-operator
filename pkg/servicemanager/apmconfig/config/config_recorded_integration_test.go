/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	apmconfigv1beta1 "github.com/oracle/oci-service-operator/api/apmconfig/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedConfigCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "apmconfig",
		Resource: "Config",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	domainID := "ocid1.apmdomain.oc1..replay"
	if mode == ocireplay.ModeRecord {
		domainID = requiredAPMConfigEnv(t, "OCI_REPLAY_APM_DOMAIN_ID")
	}
	sdkClient, closeSession := ocireplay.OpenAPMConfigSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "config_crud.yaml"),
		metadata,
	)
	resource := &apmconfigv1beta1.Config{
		Spec: apmconfigv1beta1.ConfigSpec{
			ApmDomainId:  domainID,
			ConfigType:   "SPAN_FILTER",
			DisplayName:  "osok-replay-span-filter",
			FilterText:   `service.name = "osok-replay"`,
			Description:  "OSOK recorded APM span filter",
			FreeformTags: map[string]string{"osok-replay": "create"},
		},
	}
	client := newConfigServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*apmconfigv1beta1.Config]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  5 * time.Second,
		Timeout:       15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *apmconfigv1beta1.Config) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *apmconfigv1beta1.Config) error {
			if current.Status.DisplayName != "osok-replay-span-filter" || current.Status.FilterText != `service.name = "osok-replay"` {
				return fmt.Errorf("created Config status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *apmconfigv1beta1.Config) {
			current.Spec.FilterText = `service.name = "osok-replay-updated"`
			current.Spec.Description = "OSOK recorded APM span filter updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *apmconfigv1beta1.Config) error {
			if current.Status.FilterText != `service.name = "osok-replay-updated"` || current.Status.Description != "OSOK recorded APM span filter updated" {
				return fmt.Errorf("updated Config status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredAPMConfigEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
