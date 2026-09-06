/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package quota

import (
	"path/filepath"
	"testing"

	limitssdk "github.com/oracle/oci-go-sdk/v65/limits"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationQuotaLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "quota_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Quota OCI mock: %v", err)
		}
	})
	resource := newQuotaRuntimeTestResource()
	ocimock.InitializeResource(resource, "mock-quota")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := limitssdk.QuotasClient{BaseClient: session.BaseClient()}
	client := newQuotaRuntimeTestClient(&fakeQuotaOCIClient{createFunc: sdkClient.CreateQuota, getFunc: sdkClient.GetQuota, listFunc: sdkClient.ListQuotas, updateFunc: sdkClient.UpdateQuota, deleteFunc: sdkClient.DeleteQuota})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
