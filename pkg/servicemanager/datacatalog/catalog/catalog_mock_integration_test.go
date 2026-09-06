/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package catalog

import (
	"path/filepath"
	"testing"

	datacatalogsdk "github.com/oracle/oci-go-sdk/v65/datacatalog"
	datacatalogv1beta1 "github.com/oracle/oci-service-operator/api/datacatalog/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationCatalogLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "catalog_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close Catalog OCI mock: %v", err)
		}
	})
	resource := &datacatalogv1beta1.Catalog{}
	ocimock.InitializeResource(resource, "mock-catalog")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := datacatalogsdk.DataCatalogClient{BaseClient: session.BaseClient()}
	manager := &CatalogServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newCatalogRuntimeHooks(manager, sdkClient)
	client := wrapCatalogGeneratedClient(hooks, defaultCatalogServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datacatalogv1beta1.Catalog](buildCatalogGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
