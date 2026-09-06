/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package tenancyattachment

import (
	"path/filepath"
	"testing"

	resourceanalyticssdk "github.com/oracle/oci-go-sdk/v65/resourceanalytics"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationTenancyAttachmentLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "tenancyattachment_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close TenancyAttachment OCI mock: %v", err)
		}
	})
	resource := newTenancyAttachmentTestResource()
	ocimock.InitializeResource(resource, "mock-tenancyattachment")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := resourceanalyticssdk.TenancyAttachmentClient{BaseClient: session.BaseClient()}
	client := newTenancyAttachmentServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
