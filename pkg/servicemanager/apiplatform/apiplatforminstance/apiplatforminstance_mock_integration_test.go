/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package apiplatforminstance

import (
	"path/filepath"
	"testing"

	apiplatformsdk "github.com/oracle/oci-go-sdk/v65/apiplatform"
	apiplatformv1beta1 "github.com/oracle/oci-service-operator/api/apiplatform/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationApiPlatformInstanceLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "apiplatforminstance_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close ApiPlatformInstance OCI mock: %v", err)
		}
	})
	resource := &apiplatformv1beta1.ApiPlatformInstance{Spec: apiplatformv1beta1.ApiPlatformInstanceSpec{
		Name: "osok-replay-api-platform", CompartmentId: "ocid1.compartment.oc1..replay", Description: "synthetic API Platform instance",
	}}
	ocimock.InitializeResource(resource, "mock-apiplatforminstance")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := apiplatformsdk.ApiPlatformClient{BaseClient: session.BaseClient()}
	manager := &ApiPlatformInstanceServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newApiPlatformInstanceRuntimeHooks(manager, sdkClient)
	client := wrapApiPlatformInstanceGeneratedClient(hooks, defaultApiPlatformInstanceServiceClient{ServiceClient: generatedruntime.NewServiceClient[*apiplatformv1beta1.ApiPlatformInstance](buildApiPlatformInstanceGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
