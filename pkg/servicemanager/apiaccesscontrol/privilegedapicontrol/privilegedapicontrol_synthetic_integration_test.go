/*
Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/
package privilegedapicontrol

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	apiaccesscontrolsdk "github.com/oracle/oci-go-sdk/v65/apiaccesscontrol"
	apiaccesscontrolv1beta1 "github.com/oracle/oci-service-operator/api/apiaccesscontrol/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

type syntheticPrivilegedApiControlClient struct {
	apiaccesscontrolsdk.PrivilegedApiControlClient
	apiaccesscontrolsdk.PrivilegedApiWorkRequestClient
}

func TestSyntheticPrivilegedApiControlCreateReadDelete(t *testing.T) {
	resource := makePrivilegedApiControlResource()
	id := "ocid1.privilegedapicontrol.oc1..synthetic"
	created := mustPrivilegedApiControlJSON(t, makeSDKPrivilegedApiControl(id, resource, apiaccesscontrolsdk.PrivilegedApiControlLifecycleStateActive))
	createWR := mustPrivilegedApiControlJSON(t, makePrivilegedApiControlWorkRequest("ocid1.workrequest.oc1..syntheticcreate", apiaccesscontrolsdk.OperationTypeCreatePrivilegedApiControl, apiaccesscontrolsdk.OperationStatusSucceeded, apiaccesscontrolsdk.ActionTypeCreated, id))
	deleteWR := mustPrivilegedApiControlJSON(t, makePrivilegedApiControlWorkRequest("ocid1.workrequest.oc1..syntheticdelete", apiaccesscontrolsdk.OperationTypeDeletePrivilegedApiControl, apiaccesscontrolsdk.OperationStatusSucceeded, apiaccesscontrolsdk.ActionTypeDeleted, id))
	metadata := ocireplay.Metadata{Service: "apiaccesscontrol", Resource: "PrivilegedApiControl", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "privilegedapicontrol_synthetic_crud.yaml"), Host: "https://api-access-control.us-ashburn-1.oci.oraclecloud.com", BasePath: "20241130", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20241130/privilegedApiControls", CreatedBody: created, CreateWorkRequestBody: createWR, DeleteWorkRequestBody: deleteWR})})
	if err != nil {
		t.Fatal(err)
	}
	base := session.BaseClient()
	sdkClient := syntheticPrivilegedApiControlClient{PrivilegedApiControlClient: apiaccesscontrolsdk.PrivilegedApiControlClient{BaseClient: base}, PrivilegedApiWorkRequestClient: apiaccesscontrolsdk.PrivilegedApiWorkRequestClient{BaseClient: base}}
	client := newPrivilegedApiControlServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*apiaccesscontrolv1beta1.PrivilegedApiControl]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *apiaccesscontrolv1beta1.PrivilegedApiControl) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *apiaccesscontrolv1beta1.PrivilegedApiControl) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created PrivilegedApiControl status = %+v", current.Status)
		}
		return nil
	}})
}
func mustPrivilegedApiControlJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
