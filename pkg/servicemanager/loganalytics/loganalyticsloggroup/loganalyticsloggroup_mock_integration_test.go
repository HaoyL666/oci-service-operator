/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package loganalyticsloggroup

import (
	"path/filepath"
	"testing"

	loganalyticssdk "github.com/oracle/oci-go-sdk/v65/loganalytics"
	loganalyticsv1beta1 "github.com/oracle/oci-service-operator/api/loganalytics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: recorded composite-path CRUD, production service manager, and real OCI SDK serialization.
func TestMockIntegrationLogAnalyticsLogGroupCompositeCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "loganalyticsloggroup_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close LogAnalyticsLogGroup OCI mock: %v", err)
		}
	})
	resource := &loganalyticsv1beta1.LogAnalyticsLogGroup{}
	ocimock.InitializeResource(resource, "mock-loganalyticsloggroup")
	resource.Namespace = "<binding:loganalytics-namespace>"
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := loganalyticssdk.LogAnalyticsClient{BaseClient: session.BaseClient()}
	client := newLogAnalyticsLogGroupServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
