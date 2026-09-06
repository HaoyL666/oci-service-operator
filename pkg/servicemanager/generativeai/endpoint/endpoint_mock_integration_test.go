/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package endpoint

import (
	"path/filepath"
	"testing"

	generativeaisdk "github.com/oracle/oci-go-sdk/v65/generativeai"
	generativeaiv1beta1 "github.com/oracle/oci-service-operator/api/generativeai/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationEndpointLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "endpoint_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Endpoint OCI mock: %v", err)
		}
	})
	resource := &generativeaiv1beta1.Endpoint{Spec: generativeaiv1beta1.EndpointSpec{
		CompartmentId: "ocid1.compartment.oc1..replay", ModelId: "ocid1.generativeaimodel.oc1..replay",
		DedicatedAiClusterId: "ocid1.generativeaidedicatedaicluster.oc1..replay", DisplayName: "osok-replay-generative-ai-endpoint",
	}}
	ocimock.InitializeResource(resource, "mock-endpoint")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := generativeaisdk.GenerativeAiClient{BaseClient: session.BaseClient()}
	manager := &EndpointServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newEndpointRuntimeHooks(manager, sdkClient)
	client := wrapEndpointGeneratedClient(hooks, defaultEndpointServiceClient{ServiceClient: generatedruntime.NewServiceClient[*generativeaiv1beta1.Endpoint](buildEndpointGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
