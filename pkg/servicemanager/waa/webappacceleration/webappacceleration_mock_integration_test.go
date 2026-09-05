/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package webappacceleration

import (
	"path/filepath"
	"testing"

	waasdk "github.com/oracle/oci-go-sdk/v65/waa"
	waav1beta1 "github.com/oracle/oci-service-operator/api/waa/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationWebAppAccelerationEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "webappacceleration_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close WebAppAcceleration OCI mock: %v", err)
		}
	})
	resource := &waav1beta1.WebAppAcceleration{}
	ocimock.InitializeResource(resource, "mock-webappacceleration")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	baseClient := session.BaseClient()
	sdkClient := recordedWebAppAccelerationOCIClient{
		WaaClient:         waasdk.WaaClient{BaseClient: baseClient},
		WorkRequestClient: waasdk.WorkRequestClient{BaseClient: baseClient},
	}
	client := newWebAppAccelerationServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
