/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package pathrouteset

import (
	"path/filepath"
	"testing"

	loadbalancersdk "github.com/oracle/oci-go-sdk/v65/loadbalancer"
	loadbalancerv1beta1 "github.com/oracle/oci-service-operator/api/loadbalancer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// Contract evidence: recorded composite-path CRUD, production service manager, and real OCI SDK serialization.
func TestMockIntegrationPathRouteSetCompositeCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "pathrouteset_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close PathRouteSet OCI mock: %v", err)
		}
	})
	resource := &loadbalancerv1beta1.PathRouteSet{}
	ocimock.InitializeResource(resource, "mock-pathrouteset")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	resource.Spec.LoadBalancerId = "<ocid:1>"
	sdkClient := loadbalancersdk.LoadBalancerClient{BaseClient: session.BaseClient()}
	hooks := newPathRouteSetRuntimeHooksWithOCIClient(sdkClient)
	applyPathRouteSetRuntimeHooks(&hooks)
	manager := &PathRouteSetServiceManager{}
	client := wrapPathRouteSetGeneratedClient(hooks, defaultPathRouteSetServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*loadbalancerv1beta1.PathRouteSet](buildPathRouteSetGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
