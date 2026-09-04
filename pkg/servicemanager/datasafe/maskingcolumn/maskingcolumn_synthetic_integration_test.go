/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package maskingcolumn

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	datasafesdk "github.com/oracle/oci-go-sdk/v65/datasafe"
	datasafev1beta1 "github.com/oracle/oci-service-operator/api/datasafe/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticMaskingColumnCreateReadDelete(t *testing.T) {
	resource := makeMaskingColumnResource()
	createdBody := mustMaskingColumnSyntheticJSON(t, makeSDKMaskingColumn(testMaskingColumnKey, testMaskingPolicyID, datasafesdk.MaskingColumnLifecycleStateActive, false))
	createWorkRequest := mustMaskingColumnSyntheticJSON(t, makeMaskingColumnWorkRequest(datasafesdk.WorkRequestStatusSucceeded, datasafesdk.WorkRequestOperationTypeCreateMaskingColumn, []datasafesdk.WorkRequestResource{{Identifier: common.String(testMaskingColumnKey), ActionType: datasafesdk.WorkRequestResourceActionTypeCreated}}))
	metadata := ocireplay.Metadata{Service: "datasafe", Resource: "MaskingColumn", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "maskingcolumn_synthetic_crud.yaml"), Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteStatus: 204, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	manager := &MaskingColumnServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newMaskingColumnDefaultRuntimeHooks(sdkClient)
	applyMaskingColumnRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapMaskingColumnGeneratedClient(hooks, defaultMaskingColumnServiceClient{ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.MaskingColumn](buildMaskingColumnGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.MaskingColumn]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *datasafev1beta1.MaskingColumn) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *datasafev1beta1.MaskingColumn) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created MaskingColumn status = %+v", current.Status)
		}
		return nil
	}})
}

func mustMaskingColumnSyntheticJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
