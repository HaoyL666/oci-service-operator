/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package distributeddatabaseprivateendpoint

import (
	"path/filepath"
	"testing"

	distributeddatabasesdk "github.com/oracle/oci-go-sdk/v65/distributeddatabase"
	distributeddatabasev1beta1 "github.com/oracle/oci-service-operator/api/distributeddatabase/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationDistributedDatabasePrivateEndpointLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "distributeddatabaseprivateendpoint_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close DistributedDatabasePrivateEndpoint OCI mock: %v", err)
		}
	})
	resource := newTestDistributedDatabasePrivateEndpointResource()
	ocimock.InitializeResource(resource, "mock-distributeddatabaseprivateendpoint")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := distributeddatabasesdk.DistributedDbPrivateEndpointServiceClient{BaseClient: session.BaseClient()}
	manager := &DistributedDatabasePrivateEndpointServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newDistributedDatabasePrivateEndpointDefaultRuntimeHooks(sdkClient)
	applyDistributedDatabasePrivateEndpointRuntimeHooks(&hooks)
	client := wrapDistributedDatabasePrivateEndpointGeneratedClient(hooks, defaultDistributedDatabasePrivateEndpointServiceClient{ServiceClient: generatedruntime.NewServiceClient[*distributeddatabasev1beta1.DistributedDatabasePrivateEndpoint](buildDistributedDatabasePrivateEndpointGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
