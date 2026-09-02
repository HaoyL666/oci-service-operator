/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ingesttimerule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	loganalyticsv1beta1 "github.com/oracle/oci-service-operator/api/loganalytics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedIngestTimeRuleCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "loganalytics",
		Resource: "IngestTimeRule",
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
	tenantID := "ocid1.tenancy.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredIngestTimeRuleEnv(t, "OCI_COMPARTMENT_ID")
		tenantID = requiredIngestTimeRuleEnv(t, "OCI_REPLAY_TENANCY_ID")
	}
	sdkClient, closeSession := ocireplay.OpenLogAnalyticsSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "ingesttimerule_crud.yaml"),
		metadata,
		nil,
	)
	resource := &loganalyticsv1beta1.IngestTimeRule{
		ObjectMeta: metav1.ObjectMeta{Name: "osok-replay-ingest-time-rule", Namespace: "default"},
		Spec: loganalyticsv1beta1.IngestTimeRuleSpec{
			CompartmentId: compartmentID,
			DisplayName:   "osok-replay-ingest-time-rule",
			Description:   "OSOK recorded ingest-time rule",
			IsEnabled:     true,
			Conditions: loganalyticsv1beta1.IngestTimeRuleConditions{
				Kind:          "FIELD",
				FieldName:     "mtag",
				FieldValue:    "osok-replay",
				FieldOperator: "EQUAL",
			},
			Actions: []loganalyticsv1beta1.IngestTimeRuleAction{{
				Type:          "METRIC_EXTRACTION",
				CompartmentId: compartmentID,
				Namespace:     "osok_replay",
				MetricName:    "matched_records",
				ResourceGroup: "integration",
				Dimensions:    []string{"SOURCE_NAME"},
			}},
			FreeformTags: map[string]string{"osok-replay": "create"},
		},
	}
	client := newIngestTimeRuleServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
		tenantID,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*loganalyticsv1beta1.IngestTimeRule]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  5 * time.Second,
		Timeout:       15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *loganalyticsv1beta1.IngestTimeRule) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *loganalyticsv1beta1.IngestTimeRule) error {
			if current.Status.DisplayName != "osok-replay-ingest-time-rule" || current.Status.LifecycleState != "ACTIVE" {
				return fmt.Errorf("created IngestTimeRule status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *loganalyticsv1beta1.IngestTimeRule) {
			current.Spec.Description = "OSOK recorded ingest-time rule updated"
			current.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
		},
		ValidateUpdated: func(current *loganalyticsv1beta1.IngestTimeRule) error {
			if current.Status.Description != "OSOK recorded ingest-time rule updated" || current.Status.FreeformTags["osok-replay"] != "update" {
				return fmt.Errorf("updated IngestTimeRule status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredIngestTimeRuleEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
