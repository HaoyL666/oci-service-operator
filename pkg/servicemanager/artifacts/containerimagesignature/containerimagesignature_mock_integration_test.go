/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package containerimagesignature

import (
	"path/filepath"
	"testing"

	artifactssdk "github.com/oracle/oci-go-sdk/v65/artifacts"
	artifactsv1beta1 "github.com/oracle/oci-service-operator/api/artifacts/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationContainerImageSignatureLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "containerimagesignature_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close ContainerImageSignature OCI mock: %v", err)
		}
	})
	resource := newContainerImageSignatureResource()
	ocimock.InitializeResource(resource, "mock-containerimagesignature")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := artifactssdk.ArtifactsClient{BaseClient: session.BaseClient()}
	manager := &ContainerImageSignatureServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newContainerImageSignatureDefaultRuntimeHooks(sdkClient)
	applyContainerImageSignatureRuntimeHooks(&hooks)
	client := wrapContainerImageSignatureGeneratedClient(hooks, defaultContainerImageSignatureServiceClient{ServiceClient: generatedruntime.NewServiceClient[*artifactsv1beta1.ContainerImageSignature](buildContainerImageSignatureGeneratedRuntimeConfig(manager, hooks))})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
