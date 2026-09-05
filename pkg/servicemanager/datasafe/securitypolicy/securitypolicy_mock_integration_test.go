/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package securitypolicy

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
func TestMockIntegrationSecurityPolicyEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "securitypolicy_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close SecurityPolicy OCI mock: %v", err)
		}
	})
	resource := &datasafev1beta1.SecurityPolicy{}
	ocimock.InitializeResource(resource, "mock-securitypolicy")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	manager := &SecurityPolicyServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newSecurityPolicyRuntimeHooksWithOCIClient(sdkClient)
	applySecurityPolicyRuntimeHooks(&hooks)
	client := wrapSecurityPolicyGeneratedClient(hooks, defaultSecurityPolicyServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.SecurityPolicy](buildSecurityPolicyGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
