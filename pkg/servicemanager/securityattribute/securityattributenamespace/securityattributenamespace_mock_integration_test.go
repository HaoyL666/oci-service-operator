/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package securityattributenamespace

import (
	"path/filepath"
	"testing"

	securityattributesdk "github.com/oracle/oci-go-sdk/v65/securityattribute"
	securityattributev1beta1 "github.com/oracle/oci-service-operator/api/securityattribute/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationSecurityAttributeNamespaceLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "securityattributenamespace_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close SecurityAttributeNamespace OCI mock: %v", err)
		}
	})
	resource := &securityattributev1beta1.SecurityAttributeNamespace{Spec: securityattributev1beta1.SecurityAttributeNamespaceSpec{
		CompartmentId: "ocid1.tenancy.oc1..replay", Name: syntheticSecurityAttributeNamespaceName, Description: "synthetic create", FreeformTags: map[string]string{"osok-replay": "synthetic"},
	}}
	ocimock.InitializeResource(resource, "mock-securityattributenamespace")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := securityattributesdk.
		SecurityAttributeClient{BaseClient: session.BaseClient()}
	client := newSecurityAttributeNamespaceServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
