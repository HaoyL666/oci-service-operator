/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package namedcredential

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	managementagentsdk "github.com/oracle/oci-go-sdk/v65/managementagent"
	managementagentv1beta1 "github.com/oracle/oci-service-operator/api/managementagent/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticNamedCredentialCreateReadDelete(t *testing.T) {
	resource := makeNamedCredentialResource()
	createdBody, err := ocireplay.SyntheticJSONBody(makeSDKNamedCredential(testNamedCredentialID, resource.Spec, managementagentsdk.NamedCredentialLifecycleStateActive))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(makeNamedCredentialWorkRequest("ocid1.workrequest.oc1..syntheticcreate", managementagentsdk.OperationTypesCreateNamedcredentials, managementagentsdk.OperationStatusSucceeded, managementagentsdk.ActionTypesCreated, testNamedCredentialID))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(makeNamedCredentialWorkRequest("ocid1.workrequest.oc1..syntheticdelete", managementagentsdk.OperationTypesDeleteNamedcredentials, managementagentsdk.OperationStatusSucceeded, managementagentsdk.ActionTypesDeleted, testNamedCredentialID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "managementagent", Resource: "NamedCredential", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "namedcredential_synthetic_crud.yaml"), Host: "https://management-agent.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200202", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := managementagentsdk.ManagementAgentClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newNamedCredentialServiceClientWithOCIClient(log, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*managementagentv1beta1.NamedCredential]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *managementagentv1beta1.NamedCredential) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *managementagentv1beta1.NamedCredential) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created NamedCredential status = %+v", current.Status)
		}
		return nil
	}})
}
