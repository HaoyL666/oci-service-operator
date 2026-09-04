/*
Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/
package delegationcontrol

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	delegateaccesscontrolsdk "github.com/oracle/oci-go-sdk/v65/delegateaccesscontrol"
	delegateaccesscontrolv1beta1 "github.com/oracle/oci-service-operator/api/delegateaccesscontrol/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

type syntheticDelegationControlClient struct {
	delegateaccesscontrolsdk.DelegateAccessControlClient
	delegateaccesscontrolsdk.WorkRequestClient
}

func TestSyntheticDelegationControlCreateReadDelete(t *testing.T) {
	resource := makeDelegationControlResource()
	id := "ocid1.delegationcontrol.oc1..synthetic"
	created := mustDelegationControlJSON(t, makeSDKDelegationControl(id, resource, delegateaccesscontrolsdk.DelegationControlLifecycleStateActive))
	createWR := mustDelegationControlJSON(t, makeDelegationControlWorkRequest("ocid1.workrequest.oc1..syntheticcreate", delegateaccesscontrolsdk.OperationTypeCreateDelegationControl, delegateaccesscontrolsdk.OperationStatusSucceeded, delegateaccesscontrolsdk.ActionTypeCreated, id))
	deleteWR := mustDelegationControlJSON(t, makeDelegationControlWorkRequest("ocid1.workrequest.oc1..syntheticdelete", delegateaccesscontrolsdk.OperationTypeDeleteDelegationControl, delegateaccesscontrolsdk.OperationStatusSucceeded, delegateaccesscontrolsdk.ActionTypeDeleted, id))
	metadata := ocireplay.Metadata{Service: "delegateaccesscontrol", Resource: "DelegationControl", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "delegationcontrol_synthetic_crud.yaml"), Host: "https://delegate-access-control.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230801", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20230801/delegationControls", CreatedBody: created, CreateWorkRequestBody: createWR, DeleteWorkRequestBody: deleteWR})})
	if err != nil {
		t.Fatal(err)
	}
	base := session.BaseClient()
	sdkClient := syntheticDelegationControlClient{DelegateAccessControlClient: delegateaccesscontrolsdk.DelegateAccessControlClient{BaseClient: base}, WorkRequestClient: delegateaccesscontrolsdk.WorkRequestClient{BaseClient: base}}
	client := newDelegationControlServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*delegateaccesscontrolv1beta1.DelegationControl]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *delegateaccesscontrolv1beta1.DelegationControl) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *delegateaccesscontrolv1beta1.DelegationControl) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created DelegationControl status = %+v", current.Status)
		}
		return nil
	}})
}
func mustDelegationControlJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
