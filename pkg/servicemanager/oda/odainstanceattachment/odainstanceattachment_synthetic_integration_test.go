/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package odainstanceattachment

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	odasdk "github.com/oracle/oci-go-sdk/v65/oda"
	odav1beta1 "github.com/oracle/oci-service-operator/api/oda/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOdaInstanceAttachmentCreateReadDelete(t *testing.T) {
	resource := makeOdaInstanceAttachmentResource()
	createdBody, err := ocireplay.SyntheticJSONBody(makeSDKOdaInstanceAttachment(testOdaInstanceAttachmentID, resource, odasdk.OdaInstanceAttachmentLifecycleStateActive))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(makeSDKOdaInstanceAttachmentWorkRequest("ocid1.workrequest.oc1..syntheticcreate", odasdk.WorkRequestRequestActionCreateOdaInstanceAttachment, odasdk.WorkRequestStatusSucceeded, testOdaInstanceAttachmentID))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(makeSDKOdaInstanceAttachmentWorkRequest("ocid1.workrequest.oc1..syntheticdelete", odasdk.WorkRequestRequestActionDeleteOdaInstanceAttachment, odasdk.WorkRequestStatusSucceeded, testOdaInstanceAttachmentID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "oda", Resource: "OdaInstanceAttachment", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "odainstanceattachment_synthetic_crud.yaml"), Host: "https://digitalassistant-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190506", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20190506/odaInstances/" + testOdaInstanceID + "/attachments", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := odasdk.OdaClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newOdaInstanceAttachmentServiceClientWithOCIClient(log, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*odav1beta1.OdaInstanceAttachment]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *odav1beta1.OdaInstanceAttachment) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *odav1beta1.OdaInstanceAttachment) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created OdaInstanceAttachment status = %+v", current.Status)
		}
		return nil
	}})
}
