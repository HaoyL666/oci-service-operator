/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package skillparameter

import (
	"fmt"
	"net/http"
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

func TestSyntheticSkillParameterCreateReadDelete(t *testing.T) {
	resource := testSkillParameter()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "oda", Resource: "SkillParameter", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "skillparameter_synthetic_crud.yaml"), Host: "https://oda.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190506", Metadata: metadata, Responses: []ocireplay.SyntheticResponse{
		{StatusCode: http.StatusOK, Body: `{"items":[]}`},
		{StatusCode: http.StatusNotFound, Body: `{"code":"NotFound","message":"parameter not found"}`},
		{StatusCode: http.StatusOK, Body: createdBody},
		{StatusCode: http.StatusOK, Body: createdBody},
		{StatusCode: http.StatusOK, Body: createdBody},
		{StatusCode: http.StatusNoContent},
		{StatusCode: http.StatusNotFound, Body: `{"code":"NotFound","message":"parameter deleted"}`},
	}})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := odasdk.ManagementClient{BaseClient: session.BaseClient()}
	client := newSkillParameterServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*odav1beta1.SkillParameter]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *odav1beta1.SkillParameter) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *odav1beta1.SkillParameter) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created SkillParameter status = %+v", current.Status)
		}
		return nil
	}})
}
