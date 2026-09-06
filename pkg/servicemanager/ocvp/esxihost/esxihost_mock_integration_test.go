/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package esxihost

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
func TestMockIntegrationEsxiHostLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "esxihost_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close EsxiHost OCI mock: %v", err)
		}
	})
	resource := newEsxiHostTestResource()
	ocimock.InitializeResource(resource, "mock-esxihost")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := ocvpsdk.EsxiHostClient{BaseClient: session.BaseClient()}
	manager := &EsxiHostServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newEsxiHostRuntimeHooks(manager, sdkClient)
	client := wrapEsxiHostGeneratedClient(hooks, defaultEsxiHostServiceClient{ServiceClient: generatedruntime.NewServiceClient[*ocvpv1beta1.EsxiHost](buildEsxiHostGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
