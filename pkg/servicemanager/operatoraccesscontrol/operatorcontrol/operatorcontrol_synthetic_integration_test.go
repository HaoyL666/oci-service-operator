/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package operatorcontrol

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	operatoraccesscontrolsdk "github.com/oracle/oci-go-sdk/v65/operatoraccesscontrol"
	operatoraccesscontrolv1beta1 "github.com/oracle/oci-service-operator/api/operatoraccesscontrol/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOperatorControlCreateReadDelete(t *testing.T) {
	resource := &operatoraccesscontrolv1beta1.OperatorControl{Spec: operatoraccesscontrolv1beta1.OperatorControlSpec{
		OperatorControlName: "osok-replay-operator-control", ApproverGroupsList: []string{"ocid1.group.oc1..replay"},
		ResourceType: string(operatoraccesscontrolsdk.ResourceTypesExacc), CompartmentId: "ocid1.compartment.oc1..replay",
		Description: "synthetic operator control", NumberOfApprovers: 1,
	}}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.operatorcontrol.oc1..synthetic", "CREATED", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "operatoraccesscontrol", Resource: "OperatorControl", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "operatorcontrol_synthetic_crud.yaml"), Host: "https://operator-access-control.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200630", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := operatoraccesscontrolsdk.OperatorControlClient{BaseClient: session.BaseClient()}
	client := newOperatorControlServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*operatoraccesscontrolv1beta1.OperatorControl]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *operatoraccesscontrolv1beta1.OperatorControl) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *operatoraccesscontrolv1beta1.OperatorControl) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.OperatorControlName != resource.Spec.OperatorControlName {
				return fmt.Errorf("created OperatorControl status = %+v", current.Status)
			}
			return nil
		},
	})
}
