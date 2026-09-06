/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package limitsincreaserequest

import (
	"path/filepath"
	"testing"

	limitsincreasesdk "github.com/oracle/oci-go-sdk/v65/limitsincrease"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationLimitsIncreaseRequestLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "limitsincreaserequest_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close LimitsIncreaseRequest OCI mock: %v", err)
		}
	})
	resource := makeLimitsIncreaseRequestResource()
	ocimock.InitializeResource(resource, "mock-limitsincreaserequest")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := limitsincreasesdk.LimitsIncreaseClient{BaseClient: session.BaseClient()}
	client := testLimitsIncreaseRequestClient(&fakeLimitsIncreaseRequestOCIClient{createFn: sdkClient.CreateLimitsIncreaseRequest, getFn: sdkClient.GetLimitsIncreaseRequest, listFn: sdkClient.ListLimitsIncreaseRequests, updateFn: sdkClient.UpdateLimitsIncreaseRequest, deleteFn: sdkClient.DeleteLimitsIncreaseRequest})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
