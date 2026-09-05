/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package managementdashboard

import (
	"path/filepath"
	"testing"

	managementdashboardsdk "github.com/oracle/oci-go-sdk/v65/managementdashboard"
	managementdashboardv1beta1 "github.com/oracle/oci-service-operator/api/managementdashboard/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationManagementDashboardEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "managementdashboard_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close ManagementDashboard OCI mock: %v", err)
		}
	})
	resource := &managementdashboardv1beta1.ManagementDashboard{}
	ocimock.InitializeResource(resource, "mock-managementdashboard")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := managementdashboardsdk.DashxApisClient{BaseClient: session.BaseClient()}
	client := newManagementDashboardServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
