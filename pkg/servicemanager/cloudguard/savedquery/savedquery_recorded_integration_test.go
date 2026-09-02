/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package savedquery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	cloudguardsdk "github.com/oracle/oci-go-sdk/v65/cloudguard"
	cloudguardv1beta1 "github.com/oracle/oci-service-operator/api/cloudguard/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedSavedQueryCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "cloudguard", Resource: "SavedQuery",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredSavedQueryEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := ocireplay.OpenCloudGuardSDK(
		t, mode, filepath.Join("testdata", "recordings", "savedquery_crud.yaml"), metadata,
	)
	resource := &cloudguardv1beta1.SavedQuery{Spec: cloudguardv1beta1.SavedQuerySpec{
		DisplayName: "osok-replay-cloud-guard-query", CompartmentId: compartmentID,
		Query: "select name, pid from processes", Description: "OSOK recorded Cloud Guard saved query",
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	manager := &SavedQueryServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")}}
	hooks := newSavedQueryRuntimeHooks(manager, sdkClient)
	client := wrapSavedQueryGeneratedClient(hooks, defaultSavedQueryServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*cloudguardv1beta1.SavedQuery](
			buildSavedQueryGeneratedRuntimeConfig(manager, hooks),
		),
	})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudguardv1beta1.SavedQuery]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession,
		PollInterval: 5 * time.Second, Timeout: 15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *cloudguardv1beta1.SavedQuery) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *cloudguardv1beta1.SavedQuery) error {
			if current.Status.DisplayName != "osok-replay-cloud-guard-query" || current.Status.LifecycleState != string(cloudguardsdk.LifecycleStateActive) {
				return fmt.Errorf("created SavedQuery status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *cloudguardv1beta1.SavedQuery) {
			current.Spec.Description = "OSOK recorded Cloud Guard saved query updated"
		},
		ValidateUpdated: func(current *cloudguardv1beta1.SavedQuery) error {
			if current.Status.Description != "OSOK recorded Cloud Guard saved query updated" {
				return fmt.Errorf("updated SavedQuery status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredSavedQueryEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
