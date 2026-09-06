/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package baselineablemetric

import (
	"path/filepath"
	"testing"

	stackmonitoringsdk "github.com/oracle/oci-go-sdk/v65/stackmonitoring"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationBaselineableMetricLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "baselineablemetric_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close BaselineableMetric OCI mock: %v", err)
		}
	})
	resource := baselineableMetricResource()
	ocimock.InitializeResource(resource, "mock-baselineablemetric")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := stackmonitoringsdk.StackMonitoringClient{BaseClient: session.BaseClient()}
	client := newBaselineableMetricServiceClientWithOCIClient(sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
