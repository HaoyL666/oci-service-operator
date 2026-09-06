/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package managementstation

import (
	"path/filepath"
	"testing"

	osmanagementhubsdk "github.com/oracle/oci-go-sdk/v65/osmanagementhub"
	osmanagementhubv1beta1 "github.com/oracle/oci-service-operator/api/osmanagementhub/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationManagementStationLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "managementstation_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close ManagementStation OCI mock: %v", err)
		}
	})
	resource := testManagementStationResource()
	ocimock.InitializeResource(resource, "mock-managementstation")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := osmanagementhubsdk.ManagementStationClient{BaseClient: session.BaseClient()}
	manager := &ManagementStationServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newManagementStationDefaultRuntimeHooks(sdkClient)
	applyManagementStationRuntimeHooks(manager, &hooks)
	client := wrapManagementStationGeneratedClient(hooks, defaultManagementStationServiceClient{ServiceClient: generatedruntime.NewServiceClient[*osmanagementhubv1beta1.ManagementStation](buildManagementStationGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
