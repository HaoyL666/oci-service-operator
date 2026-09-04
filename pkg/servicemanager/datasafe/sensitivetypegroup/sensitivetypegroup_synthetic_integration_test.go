/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package sensitivetypegroup

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	datasafesdk "github.com/oracle/oci-go-sdk/v65/datasafe"
	datasafev1beta1 "github.com/oracle/oci-service-operator/api/datasafe/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	"github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticSensitiveTypeGroupCreateReadDelete(t *testing.T) {
	resource := newSensitiveTypeGroupResource()
	createdBody := mustSensitiveTypeGroupSyntheticJSON(t, sensitiveTypeGroupBody(resource, testSensitiveTypeGroupID, datasafesdk.SensitiveTypeGroupLifecycleStateActive))
	createWorkRequest := mustSensitiveTypeGroupSyntheticJSON(t, sensitiveTypeGroupWorkRequest("ocid1.workrequest.oc1..syntheticcreate", shared.OSOKAsyncPhaseCreate, datasafesdk.WorkRequestStatusSucceeded))
	deleteWorkRequest := mustSensitiveTypeGroupSyntheticJSON(t, sensitiveTypeGroupWorkRequest("ocid1.workrequest.oc1..syntheticdelete", shared.OSOKAsyncPhaseDelete, datasafesdk.WorkRequestStatusSucceeded))
	metadata := ocireplay.Metadata{Service: "datasafe", Resource: "SensitiveTypeGroup", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "sensitivetypegroup_synthetic_crud.yaml"), Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	manager := &SensitiveTypeGroupServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newSensitiveTypeGroupDefaultRuntimeHooks(sdkClient)
	applySensitiveTypeGroupRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapSensitiveTypeGroupGeneratedClient(hooks, defaultSensitiveTypeGroupServiceClient{ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.SensitiveTypeGroup](buildSensitiveTypeGroupGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.SensitiveTypeGroup]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *datasafev1beta1.SensitiveTypeGroup) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *datasafev1beta1.SensitiveTypeGroup) error {
		if current.Status.OsokStatus.Ocid == "" && current.Status.Id == "" {
			return fmt.Errorf("created SensitiveTypeGroup status = %+v", current.Status)
		}
		return nil
	}})
}

func mustSensitiveTypeGroupSyntheticJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
