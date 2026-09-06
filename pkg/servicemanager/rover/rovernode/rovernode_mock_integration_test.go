/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package rovernode

import (
	"path/filepath"
	"testing"

	roversdk "github.com/oracle/oci-go-sdk/v65/rover"
	roverv1beta1 "github.com/oracle/oci-service-operator/api/rover/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationRoverNodeLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "rovernode_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close RoverNode OCI mock: %v", err)
		}
	})
	resource := &roverv1beta1.RoverNode{Spec: roverv1beta1.RoverNodeSpec{
		DisplayName: "osok-replay-rover-node", CompartmentId: "ocid1.compartment.oc1..replay", Shape: "Rover.Node.1.168",
	}}
	ocimock.InitializeResource(resource, "mock-rovernode")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := roversdk.RoverNodeClient{BaseClient: session.BaseClient()}
	manager := &RoverNodeServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newRoverNodeRuntimeHooks(manager, sdkClient)
	client := wrapRoverNodeGeneratedClient(hooks, defaultRoverNodeServiceClient{ServiceClient: generatedruntime.NewServiceClient[*roverv1beta1.RoverNode](buildRoverNodeGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
