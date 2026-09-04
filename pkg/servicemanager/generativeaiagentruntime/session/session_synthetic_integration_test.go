/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package session

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	generativeaiagentruntimesdk "github.com/oracle/oci-go-sdk/v65/generativeaiagentruntime"
	generativeaiagentruntimev1beta1 "github.com/oracle/oci-service-operator/api/generativeaiagentruntime/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticSessionCreateReadDelete(t *testing.T) {
	resource := &generativeaiagentruntimev1beta1.Session{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"session","namespace":"default"},"spec":{"displayName":"osok replay session","description":"portable replay","agentEndpointId":"ocid1.generativeaiagentendpoint.oc1..replay"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.generativeaiagentsession.oc1..synthetic", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "generativeaiagentruntime", Resource: "Session", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "session_synthetic_crud.yaml"), Host: "https://agent-runtime.generativeai.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240531", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := generativeaiagentruntimesdk.GenerativeAiAgentRuntimeClient{BaseClient: session.BaseClient()}
	client := newSessionServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*generativeaiagentruntimev1beta1.Session]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *generativeaiagentruntimev1beta1.Session) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *generativeaiagentruntimev1beta1.Session) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created Session status = %+v", current.Status)
		}
		return nil
	}})
}
