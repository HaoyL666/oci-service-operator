/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package httpredirect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	waasv1beta1 "github.com/oracle/oci-service-operator/api/waas/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedHttpRedirectCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "waas", Resource: "HttpRedirect", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredHttpRedirectRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := ocireplay.OpenWAASRedirectSDK(t, mode, filepath.Join("testdata", "recordings", "httpredirect_crud.yaml"), metadata)
	resource := &waasv1beta1.HttpRedirect{Spec: waasv1beta1.HttpRedirectSpec{
		CompartmentId: compartmentID,
		Domain:        "osok-replay-redirect.example.com",
		DisplayName:   "osok-replay-http-redirect",
		ResponseCode:  301,
		Target:        waasv1beta1.HttpRedirectTarget{Protocol: "https", Host: "www.example.com", Path: "/{path}", Query: "{query}"},
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	client := newHttpRedirectServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*waasv1beta1.HttpRedirect]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 20 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *waasv1beta1.HttpRedirect) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *waasv1beta1.HttpRedirect) error {
			if current.Status.DisplayName != "osok-replay-http-redirect" || current.Status.ResponseCode != 301 {
				return fmt.Errorf("created HttpRedirect status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *waasv1beta1.HttpRedirect) {
			current.Spec.DisplayName = "osok-replay-http-redirect-updated"
			current.Spec.ResponseCode = 302
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *waasv1beta1.HttpRedirect) error {
			if current.Status.DisplayName != "osok-replay-http-redirect-updated" || current.Status.ResponseCode != 302 {
				return fmt.Errorf("updated HttpRedirect status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredHttpRedirectRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
