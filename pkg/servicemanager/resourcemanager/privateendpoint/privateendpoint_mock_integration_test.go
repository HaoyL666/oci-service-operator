/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package privateendpoint

import (
	"path/filepath"
	"testing"

	resourcemanagersdk "github.com/oracle/oci-go-sdk/v65/resourcemanager"
	resourcemanagerv1beta1 "github.com/oracle/oci-service-operator/api/resourcemanager/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationPrivateEndpointEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "privateendpoint_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close PrivateEndpoint OCI mock: %v", err)
		}
	})
	resource := &resourcemanagerv1beta1.PrivateEndpoint{}
	ocimock.InitializeResource(resource, "mock-privateendpoint")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := resourcemanagersdk.ResourceManagerClient{BaseClient: session.BaseClient()}
	manager := &PrivateEndpointServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newPrivateEndpointRuntimeHooks(manager, sdkClient)
	client := wrapPrivateEndpointGeneratedClient(hooks, defaultPrivateEndpointServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*resourcemanagerv1beta1.PrivateEndpoint](buildPrivateEndpointGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
