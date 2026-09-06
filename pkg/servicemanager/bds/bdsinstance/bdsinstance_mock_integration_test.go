/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package bdsinstance

import (
	"path/filepath"
	"testing"

	bdssdk "github.com/oracle/oci-go-sdk/v65/bds"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationBdsInstanceLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "bdsinstance_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close BdsInstance OCI mock: %v", err)
		}
	})
	resource := makeSpecBdsInstance()
	ocimock.InitializeResource(resource, "mock-bdsinstance")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := bdssdk.BdsClient{BaseClient: session.BaseClient()}
	client := newBdsInstanceTestManager(sdkClient).client
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
