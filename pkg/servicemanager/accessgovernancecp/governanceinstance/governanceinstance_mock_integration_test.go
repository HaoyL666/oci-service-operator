/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package governanceinstance

import (
	"path/filepath"
	"testing"

	accessgovernancecpsdk "github.com/oracle/oci-go-sdk/v65/accessgovernancecp"
	accessgovernancecpv1beta1 "github.com/oracle/oci-service-operator/api/accessgovernancecp/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationGovernanceInstanceLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "governanceinstance_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close GovernanceInstance OCI mock: %v", err)
		}
	})
	resource := &accessgovernancecpv1beta1.GovernanceInstance{
		Spec: accessgovernancecpv1beta1.GovernanceInstanceSpec{
			DisplayName:      syntheticGovernanceInstanceName,
			LicenseType:      string(accessgovernancecpsdk.LicenseTypeNewLicense),
			TenancyNamespace: "synthetic-namespace",
			CompartmentId:    "ocid1.compartment.oc1..replay",
			IdcsAccessToken:  "synthetic-administrator-token",
			Description:      "synthetic create",
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	ocimock.InitializeResource(resource, "mock-governanceinstance")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := accessgovernancecpsdk.AccessGovernanceCPClient{BaseClient: session.BaseClient()}
	client := newGovernanceInstanceServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")},
		sdkClient,
	)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
