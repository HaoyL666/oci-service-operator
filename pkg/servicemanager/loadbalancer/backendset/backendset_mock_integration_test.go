/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package backendset

import (
	"path/filepath"
	"testing"

	loadbalancersdk "github.com/oracle/oci-go-sdk/v65/loadbalancer"
	loadbalancerv1beta1 "github.com/oracle/oci-service-operator/api/loadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// Contract evidence: recorded composite-path CRUD, production service manager, and real OCI SDK serialization.
func TestMockIntegrationBackendSetCompositeCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "backendset_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close BackendSet OCI mock: %v", err)
		}
	})
	resource := &loadbalancerv1beta1.BackendSet{}
	ocimock.InitializeResource(resource, "mock-backendset")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	resource.Spec.LoadBalancerId = "<ocid:1>"
	sdkClient := loadbalancersdk.LoadBalancerClient{BaseClient: session.BaseClient()}
	hooks := newBackendSetRuntimeHooksWithOCIClient(sdkClient)
	applyBackendSetRuntimeHooks(&hooks)
	client := defaultBackendSetServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*loadbalancerv1beta1.BackendSet](buildBackendSetGeneratedRuntimeConfig(&BackendSetServiceManager{}, hooks)),
	}
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
