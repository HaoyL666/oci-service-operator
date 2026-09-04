/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package sensitivecolumn

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

func TestSyntheticSensitiveColumnCreateReadDelete(t *testing.T) {
	resource := makeSensitiveColumnResource()
	createdBody := mustSensitiveColumnSyntheticJSON(t, makeSDKSensitiveColumn(testSensitiveColumnKey, testSensitiveDataModelID, datasafesdk.SensitiveColumnLifecycleStateActive))
	createWorkRequest := mustSensitiveColumnSyntheticJSON(t, makeSensitiveColumnWorkRequest(datasafesdk.WorkRequestStatusSucceeded, datasafesdk.WorkRequestOperationTypeCreateSensitiveColumn, []datasafesdk.WorkRequestResource{{Identifier: common.String(testSensitiveColumnKey), ActionType: datasafesdk.WorkRequestResourceActionTypeCreated}}))
	metadata := ocireplay.Metadata{Service: "datasafe", Resource: "SensitiveColumn", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "sensitivecolumn_synthetic_crud.yaml"), Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteStatus: 204, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	manager := &SensitiveColumnServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newSensitiveColumnDefaultRuntimeHooks(sdkClient)
	applySensitiveColumnRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapSensitiveColumnGeneratedClient(hooks, defaultSensitiveColumnServiceClient{ServiceClient: generatedruntime.NewServiceClient[*datasafev1beta1.SensitiveColumn](buildSensitiveColumnGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.SensitiveColumn]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *datasafev1beta1.SensitiveColumn) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *datasafev1beta1.SensitiveColumn) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created SensitiveColumn status = %+v", current.Status)
		}
		return nil
	}})
}

func mustSensitiveColumnSyntheticJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
