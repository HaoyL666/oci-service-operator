/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package agentdependency

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

func TestSyntheticAgentDependencyCreateReadDelete(t *testing.T) {
	resource := &cloudbridgev1beta1.AgentDependency{Spec: cloudbridgev1beta1.AgentDependencySpec{
		DisplayName: "osok-replay-agent-dependency", DependencyName: "VDDK", CompartmentId: "ocid1.compartment.oc1..replay",
		Namespace: "replay", Bucket: "replay-agent-dependencies", ObjectName: "vddk.zip", DependencyVersion: "1.0",
	}}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.ocbagentdependency.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "cloudbridge", Resource: "AgentDependency", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "agentdependency_synthetic_crud.yaml"), Host: "https://cloudbridge.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220509", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := cloudbridgesdk.OcbAgentSvcClient{BaseClient: session.BaseClient()}
	manager := &AgentDependencyServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newAgentDependencyRuntimeHooks(manager, sdkClient)
	client := wrapAgentDependencyGeneratedClient(hooks, defaultAgentDependencyServiceClient{ServiceClient: generatedruntime.NewServiceClient[*cloudbridgev1beta1.AgentDependency](buildAgentDependencyGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudbridgev1beta1.AgentDependency]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *cloudbridgev1beta1.AgentDependency) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *cloudbridgev1beta1.AgentDependency) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.DisplayName != resource.Spec.DisplayName {
				return fmt.Errorf("created AgentDependency status = %+v", current.Status)
			}
			return nil
		},
	})
}
