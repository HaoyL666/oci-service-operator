/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package loganalyticsentity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	loganalyticsv1beta1 "github.com/oracle/oci-service-operator/api/loganalytics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRecordedLogAnalyticsEntityCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loganalytics", Resource: "LogAnalyticsEntity", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	compartmentID, namespaceName := "ocid1.tenancy.oc1..replay", "replay-namespace"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredEntityRecordingEnv(t, "OCI_TENANCY_ID")
		namespaceName = requiredEntityRecordingEnv(t, "OCI_REPLAY_LOG_ANALYTICS_NAMESPACE")
	}
	sdkClient, closeSession := ocireplay.OpenLogAnalyticsSDK(t, mode, filepath.Join("testdata", "recordings", "loganalyticsentity_crud.yaml"), metadata, map[string]string{"loganalytics-namespace": namespaceName})
	resource := &loganalyticsv1beta1.LogAnalyticsEntity{
		ObjectMeta: metav1.ObjectMeta{Name: "osok-replay-log-analytics-entity", Namespace: "default"},
		Spec: loganalyticsv1beta1.LogAnalyticsEntitySpec{
			Name:           "osok-replay-log-analytics-entity",
			CompartmentId:  compartmentID,
			EntityTypeName: "Palo Alto Networks",
			TimezoneRegion: "UTC",
			Hostname:       "osok-replay-create.example.com",
			FreeformTags:   map[string]string{"osok-replay": "create"},
		},
	}
	hooks := newLogAnalyticsEntityRuntimeHooksWithOCIClient(sdkClient)
	applyLogAnalyticsEntityRuntimeHooks(&hooks, sdkClient, nil)
	manager := &LogAnalyticsEntityServiceManager{}
	client := wrapLogAnalyticsEntityGeneratedClient(hooks, defaultLogAnalyticsEntityServiceClient{ServiceClient: generatedruntime.NewServiceClient[*loganalyticsv1beta1.LogAnalyticsEntity](buildLogAnalyticsEntityGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*loganalyticsv1beta1.LogAnalyticsEntity]{
		Mode: mode, Resource: resource, Client: client, CloseSession: closeSession, PollInterval: 5 * time.Second, Timeout: 15 * time.Minute, CleanupTimeout: 30 * time.Second,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *loganalyticsv1beta1.LogAnalyticsEntity) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *loganalyticsv1beta1.LogAnalyticsEntity) error {
			if current.Status.Name != "osok-replay-log-analytics-entity" || current.Status.Id == "" {
				return fmt.Errorf("created LogAnalyticsEntity status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *loganalyticsv1beta1.LogAnalyticsEntity) {
			current.Spec.Hostname = "osok-replay-update.example.com"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *loganalyticsv1beta1.LogAnalyticsEntity) error {
			if current.Status.Hostname != "osok-replay-update.example.com" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated LogAnalyticsEntity status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredEntityRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
