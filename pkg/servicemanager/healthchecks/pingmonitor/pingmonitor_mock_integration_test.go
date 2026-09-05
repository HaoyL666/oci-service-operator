/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package pingmonitor

import (
	"path/filepath"
	"testing"

	healthcheckssdk "github.com/oracle/oci-go-sdk/v65/healthchecks"
	healthchecksv1beta1 "github.com/oracle/oci-service-operator/api/healthchecks/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationPingMonitorEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "pingmonitor_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close PingMonitor OCI mock: %v", err)
		}
	})
	resource := &healthchecksv1beta1.PingMonitor{}
	ocimock.InitializeResource(resource, "mock-pingmonitor")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := healthcheckssdk.HealthChecksClient{BaseClient: session.BaseClient()}
	client := newTestPingMonitorClient(sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
