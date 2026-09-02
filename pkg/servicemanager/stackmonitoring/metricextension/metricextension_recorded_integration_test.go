/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package metricextension

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	stackmonitoringv1beta1 "github.com/oracle/oci-service-operator/api/stackmonitoring/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedMetricExtensionCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "stackmonitoring",
		Resource: "MetricExtension",
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
		compartmentID = requiredMetricExtensionEnv(t, "OCI_COMPARTMENT_ID")
	}
	sdkClient, closeSession := ocireplay.OpenStackMonitoringSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "metricextension_crud.yaml"),
		metadata,
	)
	resource := &stackmonitoringv1beta1.MetricExtension{
		Spec: stackmonitoringv1beta1.MetricExtensionSpec{
			Name:                  "ME_OSOK_REPLAY_GC_TIME",
			DisplayName:           "OSOK replay garbage collection time",
			ResourceType:          "weblogic_j2eeserver",
			CompartmentId:         compartmentID,
			CollectionRecurrences: "FREQ=DAILY;INTERVAL=1",
			MetricList: []stackmonitoringv1beta1.MetricExtensionMetricList{
				{Name: "ServerName", DataType: "STRING", DisplayName: "Server name", IsDimension: true},
				{Name: "ServerRuntime", DataType: "STRING", DisplayName: "Server runtime", IsDimension: true},
				{Name: "TotalGCExecTime", DataType: "NUMBER", DisplayName: "Total garbage collection execution time", MetricCategory: "UTILIZATION", Unit: "milliseconds"},
			},
			QueryProperties: stackmonitoringv1beta1.MetricExtensionQueryProperties{
				CollectionMethod:       "JMX",
				ManagedBeanQuery:       "java.lang:Location=%name%,type=GarbageCollector,*",
				JmxAttributes:          "CollectionTime",
				IdentityMetric:         "name;Location",
				IsMetricServiceEnabled: false,
			},
			Description: "OSOK recorded metric extension",
		},
	}
	client := newMetricExtensionServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*stackmonitoringv1beta1.MetricExtension]{
		Mode:          mode,
		Resource:      resource,
		Client:        client,
		CloseSession:  closeSession,
		PollInterval:  5 * time.Second,
		Timeout:       15 * time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity: func(current *stackmonitoringv1beta1.MetricExtension) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *stackmonitoringv1beta1.MetricExtension) error {
			if current.Status.DisplayName != "OSOK replay garbage collection time" || current.Status.LifecycleState != "ACTIVE" {
				return fmt.Errorf("created MetricExtension status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *stackmonitoringv1beta1.MetricExtension) {
			current.Spec.DisplayName = "OSOK replay metric extension updated"
			current.Spec.Description = "OSOK recorded metric extension updated"
		},
		ValidateUpdated: func(current *stackmonitoringv1beta1.MetricExtension) error {
			if current.Status.DisplayName != "OSOK replay metric extension updated" || current.Status.Description != "OSOK recorded metric extension updated" {
				return fmt.Errorf("updated MetricExtension status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredMetricExtensionEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
