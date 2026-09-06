/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package cluster

import (
	"path/filepath"
	"testing"

	ocvpsdk "github.com/oracle/oci-go-sdk/v65/ocvp"
	ocvpv1beta1 "github.com/oracle/oci-service-operator/api/ocvp/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationClusterLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "cluster_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Cluster OCI mock: %v", err)
		}
	})
	resource := newClusterTestResource()
	ocimock.InitializeResource(resource, "mock-cluster")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := ocvpsdk.ClusterClient{BaseClient: session.BaseClient()}
	manager := &ClusterServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newClusterRuntimeHooks(manager, sdkClient)
	client := wrapClusterGeneratedClient(hooks, defaultClusterServiceClient{ServiceClient: generatedruntime.NewServiceClient[*ocvpv1beta1.Cluster](buildClusterGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
