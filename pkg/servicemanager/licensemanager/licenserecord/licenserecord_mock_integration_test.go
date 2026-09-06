/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package licenserecord

import (
	"path/filepath"
	"testing"

	licensemanagersdk "github.com/oracle/oci-go-sdk/v65/licensemanager"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationLicenseRecordLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "licenserecord_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close LicenseRecord OCI mock: %v", err)
		}
	})
	resource := makeLicenseRecordResource()
	ocimock.InitializeResource(resource, "mock-licenserecord")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	resource.Annotations[LicenseRecordProductLicenseIDAnnotation] = "<ocid:1>"
	sdkClient := licensemanagersdk.LicenseManagerClient{BaseClient: session.BaseClient()}
	client := newLicenseRecordServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
