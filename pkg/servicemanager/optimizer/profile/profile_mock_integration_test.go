/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package profile

import (
	"path/filepath"
	"testing"

	optimizersdk "github.com/oracle/oci-go-sdk/v65/optimizer"
	optimizerv1beta1 "github.com/oracle/oci-service-operator/api/optimizer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationProfileLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "profile_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Profile OCI mock: %v", err)
		}
	})
	resource := newProfileRuntimeTestResource()
	ocimock.InitializeResource(resource, "mock-profile")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := optimizersdk.OptimizerClient{BaseClient: session.BaseClient()}
	manager := &ProfileServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newProfileDefaultRuntimeHooks(sdkClient)
	applyProfileRuntimeHooks(&hooks)
	client := wrapProfileGeneratedClient(hooks, defaultProfileServiceClient{ServiceClient: generatedruntime.NewServiceClient[*optimizerv1beta1.Profile](buildProfileGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
