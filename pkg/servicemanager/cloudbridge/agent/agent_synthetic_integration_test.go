/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package agent

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	cloudbridgesdk "github.com/oracle/oci-go-sdk/v65/cloudbridge"
	cloudbridgev1beta1 "github.com/oracle/oci-service-operator/api/cloudbridge/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticAgentCreateReadDelete(t *testing.T) {
	resource := &cloudbridgev1beta1.Agent{Spec: cloudbridgev1beta1.AgentSpec{
		DisplayName: "osok-replay-cloudbridge-agent", AgentType: "APPLIANCE", AgentVersion: "1.0",
		CompartmentId: "ocid1.compartment.oc1..replay", EnvironmentId: "ocid1.ocbenvironment.oc1..replay", OsVersion: "Oracle Linux 8",
	}}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.ocbagent.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "cloudbridge", Resource: "Agent", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "agent_synthetic_crud.yaml"), Host: "https://cloudbridge.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220509", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := cloudbridgesdk.OcbAgentSvcClient{BaseClient: session.BaseClient()}
	manager := &AgentServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newAgentRuntimeHooks(manager, sdkClient)
	client := wrapAgentGeneratedClient(hooks, defaultAgentServiceClient{ServiceClient: generatedruntime.NewServiceClient[*cloudbridgev1beta1.Agent](buildAgentGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudbridgev1beta1.Agent]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *cloudbridgev1beta1.Agent) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *cloudbridgev1beta1.Agent) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.DisplayName != resource.Spec.DisplayName {
				return fmt.Errorf("created Agent status = %+v", current.Status)
			}
			return nil
		},
	})
}
