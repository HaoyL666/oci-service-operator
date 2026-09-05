/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package librarymaskingformat

import (
	"path/filepath"
	"testing"

	datasafesdk "github.com/oracle/oci-go-sdk/v65/datasafe"
	datasafev1beta1 "github.com/oracle/oci-service-operator/api/datasafe/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationLibraryMaskingFormatEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "librarymaskingformat_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close LibraryMaskingFormat OCI mock: %v", err)
		}
	})
	resource := &datasafev1beta1.LibraryMaskingFormat{}
	ocimock.InitializeResource(resource, "mock-librarymaskingformat")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	manager := &LibraryMaskingFormatServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newLibraryMaskingFormatDefaultRuntimeHooks(sdkClient)
	applyLibraryMaskingFormatRuntimeHooks(&hooks)
	client := wrapLibraryMaskingFormatGeneratedClient(hooks, defaultLibraryMaskingFormatServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.LibraryMaskingFormat](buildLibraryMaskingFormatGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
