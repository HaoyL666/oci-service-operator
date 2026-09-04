/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package aidataplatform

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	aidataplatformsdk "github.com/oracle/oci-go-sdk/v65/aidataplatform"
	aidataplatformv1beta1 "github.com/oracle/oci-service-operator/api/aidataplatform/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticAiDataPlatformCreateReadDelete(t *testing.T) {
	resource := newAiDataPlatformResource()
	resourceID := "ocid1.aidataplatform.oc1..synthetic"
	createdBody := mustAiDataPlatformJSON(t, aiDataPlatformBody(resourceID, resource.Spec.CompartmentId, resource.Spec.DisplayName, aidataplatformsdk.AiDataPlatformLifecycleStateActive))
	createWorkRequest := mustAiDataPlatformJSON(t, aiDataPlatformWorkRequest("ocid1.workrequest.oc1..syntheticcreate", aidataplatformsdk.OperationTypeCreateDataLake, aidataplatformsdk.OperationStatusSucceeded, aidataplatformsdk.ActionTypeCreated, resourceID))
	deleteWorkRequest := mustAiDataPlatformJSON(t, aiDataPlatformWorkRequest("ocid1.workrequest.oc1..syntheticdelete", aidataplatformsdk.OperationTypeDeleteDataLake, aidataplatformsdk.OperationStatusSucceeded, aidataplatformsdk.ActionTypeDeleted, resourceID))
	metadata := ocireplay.Metadata{Service: "aidataplatform", Resource: "AiDataPlatform", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "aidataplatform_synthetic_crud.yaml"), Host: "https://aidataplatform.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240831", Metadata: metadata,
		Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20240831/aiDataPlatforms", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := aidataplatformsdk.AiDataPlatformClient{BaseClient: session.BaseClient()}
	client := newAiDataPlatformServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*aidataplatformv1beta1.AiDataPlatform]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *aidataplatformv1beta1.AiDataPlatform) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *aidataplatformv1beta1.AiDataPlatform) error {
			if current.Status.Id == "" {
				return fmt.Errorf("created AiDataPlatform status = %+v", current.Status)
			}
			return nil
		},
	})
}

func mustAiDataPlatformJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
