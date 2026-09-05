/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package cabundle

import (
	"path/filepath"
	"testing"

	certificatesmanagementsdk "github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
	certificatesmanagementv1beta1 "github.com/oracle/oci-service-operator/api/certificatesmanagement/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationCaBundleEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "cabundle_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close CaBundle OCI mock: %v", err)
		}
	})
	resource := &certificatesmanagementv1beta1.CaBundle{}
	ocimock.InitializeResource(resource, "mock-cabundle")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := certificatesmanagementsdk.CertificatesManagementClient{BaseClient: session.BaseClient()}
	client := newCaBundleServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
