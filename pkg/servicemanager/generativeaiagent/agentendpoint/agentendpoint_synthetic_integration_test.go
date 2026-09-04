/*
Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/
package agentendpoint

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	generativeaiagentsdk "github.com/oracle/oci-go-sdk/v65/generativeaiagent"
	generativeaiagentv1beta1 "github.com/oracle/oci-service-operator/api/generativeaiagent/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticAgentEndpointCreateReadDelete(t *testing.T) {
	resource := makeAgentEndpointResource()
	id := "ocid1.generativeaiagentendpoint.oc1..synthetic"
	created := mustAgentEndpointJSON(t, makeSDKAgentEndpoint(t, id, resource, generativeaiagentsdk.AgentEndpointLifecycleStateActive))
	createWR := mustAgentEndpointJSON(t, makeAgentEndpointWorkRequest("ocid1.workrequest.oc1..syntheticcreate", generativeaiagentsdk.OperationTypeCreateAgentEndpoint, generativeaiagentsdk.OperationStatusSucceeded, generativeaiagentsdk.ActionTypeCreated, id))
	deleteWR := mustAgentEndpointJSON(t, makeAgentEndpointWorkRequest("ocid1.workrequest.oc1..syntheticdelete", generativeaiagentsdk.OperationTypeDeleteAgentEndpoint, generativeaiagentsdk.OperationStatusSucceeded, generativeaiagentsdk.ActionTypeDeleted, id))
	metadata := ocireplay.Metadata{Service: "generativeaiagent", Resource: "AgentEndpoint", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "agentendpoint_synthetic_crud.yaml"), Host: "https://generative-ai-agent.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240531", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20240531/agentEndpoints", CreatedBody: created, CreateWorkRequestBody: createWR, DeleteWorkRequestBody: deleteWR})})
	if err != nil {
		t.Fatal(err)
	}
	client := newAgentEndpointServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, generativeaiagentsdk.GenerativeAiAgentClient{BaseClient: session.BaseClient()})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*generativeaiagentv1beta1.AgentEndpoint]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *generativeaiagentv1beta1.AgentEndpoint) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *generativeaiagentv1beta1.AgentEndpoint) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created AgentEndpoint status = %+v", current.Status)
		}
		return nil
	}})
}

func mustAgentEndpointJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
