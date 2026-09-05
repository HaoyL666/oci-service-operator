/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package approvaltemplate

import (
	"path/filepath"
	"testing"

	lockboxsdk "github.com/oracle/oci-go-sdk/v65/lockbox"
	lockboxv1beta1 "github.com/oracle/oci-service-operator/api/lockbox/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: sanitized live CRUD trace, production service manager, and real OCI SDK serialization.
func TestMockIntegrationApprovalTemplateEvidenceCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "approvaltemplate_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close ApprovalTemplate OCI mock: %v", err)
		}
	})
	resource := &lockboxv1beta1.ApprovalTemplate{}
	ocimock.InitializeResource(resource, "mock-approvaltemplate")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := lockboxsdk.LockboxClient{BaseClient: session.BaseClient()}
	manager := &ApprovalTemplateServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("mock-integration")}}
	hooks := newApprovalTemplateRuntimeHooksWithOCIClient(sdkClient)
	applyApprovalTemplateRuntimeHooks(manager, &hooks)
	client := wrapApprovalTemplateGeneratedClient(hooks, defaultApprovalTemplateServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*lockboxv1beta1.ApprovalTemplate](buildApprovalTemplateGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
