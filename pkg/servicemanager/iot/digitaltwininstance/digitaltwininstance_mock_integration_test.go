/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package digitaltwininstance

import (
	"path/filepath"
	"testing"

	iotsdk "github.com/oracle/oci-go-sdk/v65/iot"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationDigitalTwinInstanceLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "digitaltwininstance_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close DigitalTwinInstance OCI mock: %v", err)
		}
	})
	resource := makeDigitalTwinInstanceResource()
	ocimock.InitializeResource(resource, "mock-digitaltwininstance")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := iotsdk.IotClient{BaseClient: session.BaseClient()}
	client := newDigitalTwinInstanceServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
