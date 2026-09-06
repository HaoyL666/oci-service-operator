/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package backend

import (
	"path/filepath"
	"testing"

	loadbalancersdk "github.com/oracle/oci-go-sdk/v65/loadbalancer"
	loadbalancerv1beta1 "github.com/oracle/oci-service-operator/api/loadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// Contract evidence: recorded composite-path CRUD, production service manager, and real OCI SDK serialization.
func TestMockIntegrationBackendCompositeCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "backend_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Backend OCI mock: %v", err)
		}
	})
	resource := &loadbalancerv1beta1.Backend{}
	ocimock.InitializeResource(resource, "mock-backend")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	resource.Spec.LoadBalancerId = "<ocid:1>"
	resource.Spec.BackendSetName = "osok_replay_backend_set"
	sdkClient := loadbalancersdk.LoadBalancerClient{BaseClient: session.BaseClient()}
	hooks := newBackendRuntimeHooksWithOCIClient(sdkClient)
	applyBackendRuntimeHooks(&hooks)
	manager := &BackendServiceManager{}
	client := wrapBackendGeneratedClient(hooks, defaultBackendServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*loadbalancerv1beta1.Backend](buildBackendGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
