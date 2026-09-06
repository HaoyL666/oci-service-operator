/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package operatorcontrol

import (
	"path/filepath"
	"testing"

	operatoraccesscontrolsdk "github.com/oracle/oci-go-sdk/v65/operatoraccesscontrol"
	operatoraccesscontrolv1beta1 "github.com/oracle/oci-service-operator/api/operatoraccesscontrol/v1beta1"
	"github.com/oracle/oci-service-operator/internal/integration/ocimock"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Contract evidence: synthetic OCI-compatible responses, production service manager, and real OCI SDK serialization.
func TestMockIntegrationOperatorControlLifecycleCRUD(t *testing.T) {
	t.Parallel()
	session, evidence, err := ocimock.OpenEvidenceCRUD(filepath.Join("testdata", "recordings", "operatorcontrol_synthetic_crud.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close OperatorControl OCI mock: %v", err)
		}
	})
	resource := &operatoraccesscontrolv1beta1.OperatorControl{Spec: operatoraccesscontrolv1beta1.OperatorControlSpec{
		OperatorControlName: "osok-replay-operator-control", ApproverGroupsList: []string{"ocid1.group.oc1..replay"},
		ResourceType: string(operatoraccesscontrolsdk.ResourceTypesExacc), CompartmentId: "ocid1.compartment.oc1..replay",
		Description: "synthetic operator control", NumberOfApprovers: 1,
	}}
	ocimock.InitializeResource(resource, "mock-operatorcontrol")
	if err := evidence.DecodeCreateSpec(&resource.Spec); err != nil {
		t.Fatal(err)
	}
	sdkClient := operatoraccesscontrolsdk.OperatorControlClient{BaseClient: session.BaseClient()}
	client := newOperatorControlServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	if err := ocimock.RunEvidenceLifecycle(resource, &resource.Spec, client, evidence); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
