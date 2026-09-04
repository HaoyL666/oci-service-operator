/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package vbsinstance

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	vbsinstsdk "github.com/oracle/oci-go-sdk/v65/vbsinst"
	vbsinstv1beta1 "github.com/oracle/oci-service-operator/api/vbsinst/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticVbsInstanceCreateReadDelete(t *testing.T) {
	resource := makeVbsInstanceResource()
	resourceID := "ocid1.vbsinstance.oc1..synthetic"
	createdBody, err := ocireplay.SyntheticJSONBody(makeSDKVbsInstance(resourceID, resource, vbsinstsdk.LifecycleStateActive))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(makeVbsInstanceWorkRequest("ocid1.workrequest.oc1..syntheticcreate", vbsinstsdk.OperationTypeCreateVbsInstance, vbsinstsdk.OperationStatusSucceeded, vbsinstsdk.ActionTypeCreated, resourceID))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(makeVbsInstanceWorkRequest("ocid1.workrequest.oc1..syntheticdelete", vbsinstsdk.OperationTypeDeleteVbsInstance, vbsinstsdk.OperationStatusSucceeded, vbsinstsdk.ActionTypeDeleted, resourceID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "vbsinst", Resource: "VbsInstance", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "vbsinstance_synthetic_crud.yaml"), Host: "https://vbstudio.us-ashburn-1.ocp.oraclecloud.com", BasePath: "20180828", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20180828/vbsInstances", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	client := newVbsInstanceServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, vbsinstsdk.VbsInstanceClient{BaseClient: session.BaseClient()})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*vbsinstv1beta1.VbsInstance]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *vbsinstv1beta1.VbsInstance) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *vbsinstv1beta1.VbsInstance) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created VbsInstance status = %+v", current.Status)
		}
		return nil
	}})
}
