/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package javadownloadtoken

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	jmsjavadownloadssdk "github.com/oracle/oci-go-sdk/v65/jmsjavadownloads"
	jmsjavadownloadsv1beta1 "github.com/oracle/oci-service-operator/api/jmsjavadownloads/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticJavaDownloadTokenCreateReadDelete(t *testing.T) {
	resource := makeJavaDownloadTokenResource()
	resourceID := "ocid1.javadownloadtoken.oc1..synthetic"
	createdBody := mustJavaDownloadTokenJSON(t, makeSDKJavaDownloadToken(resourceID, resource, jmsjavadownloadssdk.LifecycleStateActive))
	createWorkRequest := mustJavaDownloadTokenJSON(t, makeJavaDownloadTokenWorkRequest("ocid1.workrequest.oc1..syntheticcreate", jmsjavadownloadssdk.OperationTypeCreateJavaDownloadToken, jmsjavadownloadssdk.OperationStatusSucceeded, jmsjavadownloadssdk.ActionTypeCreated, resourceID))
	deleteWorkRequest := mustJavaDownloadTokenJSON(t, makeJavaDownloadTokenWorkRequest("ocid1.workrequest.oc1..syntheticdelete", jmsjavadownloadssdk.OperationTypeDeleteJavaDownloadToken, jmsjavadownloadssdk.OperationStatusSucceeded, jmsjavadownloadssdk.ActionTypeDeleted, resourceID))
	metadata := ocireplay.Metadata{Service: "jmsjavadownloads", Resource: "JavaDownloadToken", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "javadownloadtoken_synthetic_crud.yaml"), Host: "https://javamanagementservice-download.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230601", Metadata: metadata,
		Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20230601/javaDownloadTokens", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest}),
	})
	if err != nil {
		t.Fatal(err)
	}
	client := newJavaDownloadTokenServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, jmsjavadownloadssdk.JavaDownloadClient{BaseClient: session.BaseClient()})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*jmsjavadownloadsv1beta1.JavaDownloadToken]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *jmsjavadownloadsv1beta1.JavaDownloadToken) bool {
			return current.Status.Id != "" || current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *jmsjavadownloadsv1beta1.JavaDownloadToken) error {
			if current.Status.Id == "" {
				return fmt.Errorf("created JavaDownloadToken status = %+v", current.Status)
			}
			return nil
		},
	})
}

func mustJavaDownloadTokenJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
