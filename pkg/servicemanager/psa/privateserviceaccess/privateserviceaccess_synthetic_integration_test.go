/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package privateserviceaccess

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	psasdk "github.com/oracle/oci-go-sdk/v65/psa"
	psav1beta1 "github.com/oracle/oci-service-operator/api/psa/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticPrivateServiceAccessCreateReadDelete(t *testing.T) {
	resource := makePrivateServiceAccessResource()
	resourceID := "ocid1.privateserviceaccess.oc1..synthetic"
	createdBody, err := ocireplay.SyntheticJSONBody(makeSDKPrivateServiceAccess(resourceID, resource, psasdk.PrivateServiceAccessLifecycleStateActive))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(makePrivateServiceAccessWorkRequest("ocid1.workrequest.oc1..syntheticcreate", psasdk.OperationTypeCreatePrivateServiceAccess, psasdk.OperationStatusSucceeded, psasdk.ActionTypeCreated, resourceID))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(makePrivateServiceAccessWorkRequest("ocid1.workrequest.oc1..syntheticdelete", psasdk.OperationTypeDeletePrivateServiceAccess, psasdk.OperationStatusSucceeded, psasdk.ActionTypeDeleted, resourceID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "psa", Resource: "PrivateServiceAccess", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "privateserviceaccess_synthetic_crud.yaml"), Host: "https://psasvc.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240301", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, WorkRequestPathMarker: "/psaWorkRequests/", PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := psasdk.PrivateServiceAccessClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newPrivateServiceAccessServiceClientWithOCIClient(log, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*psav1beta1.PrivateServiceAccess]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *psav1beta1.PrivateServiceAccess) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *psav1beta1.PrivateServiceAccess) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created PrivateServiceAccess status = %+v", current.Status)
		}
		return nil
	}})
}
