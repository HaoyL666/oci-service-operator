/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package sdmmaskingpolicydifference

import (
	"path/filepath"
	"testing"

	datasafesdk "github.com/oracle/oci-go-sdk/v65/datasafe"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationSdmMaskingPolicyDifferenceLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "sdmmaskingpolicydifference_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close SdmMaskingPolicyDifference OCI mock: %v", err)
		}
	})
	resource := newSdmMaskingPolicyDifferenceTestResource()
	ocimock.InitializeResource(resource, "mock-sdmmaskingpolicydifference")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	hooks := newSdmMaskingPolicyDifferenceDefaultRuntimeHooks(sdkClient)
	applySdmMaskingPolicyDifferenceRuntimeHooks(&hooks)
	client := newSdmMaskingPolicyDifferenceRuntimeTestClient(hooks)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
