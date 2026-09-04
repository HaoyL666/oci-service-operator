/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package workspace

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	dataintegrationsdk "github.com/oracle/oci-go-sdk/v65/dataintegration"
	dataintegrationv1beta1 "github.com/oracle/oci-service-operator/api/dataintegration/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticWorkspaceCreateReadDelete(t *testing.T) {
	resource := &dataintegrationv1beta1.Workspace{}
	if err := json.Unmarshal([]byte(`{"metadata":{"name":"workspace","namespace":"default"},"spec":{"displayName":"osok-replay-workspace","compartmentId":"ocid1.compartment.oc1..replay","description":"portable replay"}}`), resource); err != nil {
		t.Fatal(err)
	}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.disworkspace.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "dataintegration", Resource: "Workspace", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "workspace_synthetic_crud.yaml"), Host: "https://dataintegration.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200430", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, EmptyCollectionBody: `[]`, PresentCollectionBody: `[` + createdBody + `]`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := dataintegrationsdk.DataIntegrationClient{BaseClient: session.BaseClient()}
	manager := &WorkspaceServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newWorkspaceRuntimeHooks(manager, sdkClient)
	client := wrapWorkspaceGeneratedClient(hooks, defaultWorkspaceServiceClient{ServiceClient: generatedruntime.NewServiceClient[*dataintegrationv1beta1.Workspace](buildWorkspaceGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*dataintegrationv1beta1.Workspace]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *dataintegrationv1beta1.Workspace) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *dataintegrationv1beta1.Workspace) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created Workspace status = %+v", current.Status)
		}
		return nil
	}})
}
