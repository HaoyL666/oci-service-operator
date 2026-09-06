/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package maskingpolicy

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

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationMaskingPolicyLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "maskingpolicy_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close MaskingPolicy OCI mock: %v", err)
		}
	})
	resource := &datasafev1beta1.MaskingPolicy{}
	ocimock.InitializeResource(resource, "mock-maskingpolicy")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	manager := &MaskingPolicyServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newMaskingPolicyRuntimeHooks(manager, sdkClient)
	client := wrapMaskingPolicyGeneratedClient(hooks, defaultMaskingPolicyServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.MaskingPolicy](buildMaskingPolicyGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
