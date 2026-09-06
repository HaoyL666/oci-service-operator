/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package unifiedauditpolicy

import (
	"encoding/json"
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
func TestMockIntegrationUnifiedAuditPolicyLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "unifiedauditpolicy_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close UnifiedAuditPolicy OCI mock: %v", err)
		}
	})
	resource := &datasafev1beta1.UnifiedAuditPolicy{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"osok-replay-unifiedauditpolicy"},"spec":{"securityPolicyId":"ocid1.securitypolicy.oc1..synthetic","unifiedAuditPolicyDefinitionId":"ocid1.unifiedauditpolicydefinition.oc1..synthetic","compartmentId":"ocid1.compartment.oc1..synthetic","status":"DISABLED","conditions":[{"entitySelection":"ALL_USERS","operationStatus":"ALL"}],"displayName":"osok-replay-unified-audit-policy"}}`), resource); err != nil {
		t.Fatal(err)
	}
	ocimock.InitializeResource(resource, "mock-unifiedauditpolicy")
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	manager := &UnifiedAuditPolicyServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newUnifiedAuditPolicyRuntimeHooks(manager, sdkClient)
	client := wrapUnifiedAuditPolicyGeneratedClient(hooks, defaultUnifiedAuditPolicyServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.UnifiedAuditPolicy](buildUnifiedAuditPolicyGeneratedRuntimeConfig(manager, hooks)),
	})
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
