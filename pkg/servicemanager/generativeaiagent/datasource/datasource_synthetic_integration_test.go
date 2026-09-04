/*
Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/
package datasource

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

func TestSyntheticDataSourceCreateReadDelete(t *testing.T) {
	resource := makeDataSourceResource()
	id := "ocid1.generativeaidatasource.oc1..synthetic"
	created := mustDataSourceJSON(t, makeSDKDataSource(id, resource, generativeaiagentsdk.DataSourceLifecycleStateActive))
	createWR := mustDataSourceJSON(t, makeDataSourceWorkRequest("ocid1.workrequest.oc1..syntheticcreate", generativeaiagentsdk.OperationTypeCreateDataSource, generativeaiagentsdk.OperationStatusSucceeded, generativeaiagentsdk.ActionTypeCreated, id))
	deleteWR := mustDataSourceJSON(t, makeDataSourceWorkRequest("ocid1.workrequest.oc1..syntheticdelete", generativeaiagentsdk.OperationTypeDeleteDataSource, generativeaiagentsdk.OperationStatusSucceeded, generativeaiagentsdk.ActionTypeDeleted, id))
	metadata := ocireplay.Metadata{Service: "generativeaiagent", Resource: "DataSource", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "datasource_synthetic_crud.yaml"), Host: "https://generative-ai-agent.us-ashburn-1.oci.oraclecloud.com", BasePath: "20240531", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20240531/dataSources", CreatedBody: created, CreateWorkRequestBody: createWR, DeleteWorkRequestBody: deleteWR})})
	if err != nil {
		t.Fatal(err)
	}
	client := newDataSourceServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, generativeaiagentsdk.GenerativeAiAgentClient{BaseClient: session.BaseClient()})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*generativeaiagentv1beta1.DataSource]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *generativeaiagentv1beta1.DataSource) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *generativeaiagentv1beta1.DataSource) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created DataSource status = %+v", current.Status)
		}
		return nil
	}})
}

func mustDataSourceJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
