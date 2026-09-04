/*
Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/
package gdppipeline

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	gdpsdk "github.com/oracle/oci-go-sdk/v65/gdp"
	gdpv1beta1 "github.com/oracle/oci-service-operator/api/gdp/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticGdpPipelineCreateReadDelete(t *testing.T) {
	resource := makeGdpPipelineResource()
	id := "ocid1.gdppipeline.oc1..synthetic"
	created := mustGdpPipelineJSON(t, makeSDKGdpPipeline(id, resource, gdpsdk.GdpPipelineLifecycleStateActive))
	createWR := mustGdpPipelineJSON(t, makeGdpPipelineWorkRequest("ocid1.workrequest.oc1..syntheticcreate", gdpsdk.GdpOperationTypeCreateGdpPipeline, gdpsdk.OperationStatusSucceeded, gdpsdk.ActionTypeCreated, id))
	deleteWR := mustGdpPipelineJSON(t, makeGdpPipelineWorkRequest("ocid1.workrequest.oc1..syntheticdelete", gdpsdk.GdpOperationTypeDeleteGdpPipeline, gdpsdk.OperationStatusSucceeded, gdpsdk.ActionTypeDeleted, id))
	metadata := ocireplay.Metadata{Service: "gdp", Resource: "GdpPipeline", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "gdppipeline_synthetic_crud.yaml"), Host: "https://gdp.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230301", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20230301/gdpPipelines", WorkRequestPathMarker: "/gdpWorkRequests/", CreatedBody: created, CreateWorkRequestBody: createWR, DeleteWorkRequestBody: deleteWR})})
	if err != nil {
		t.Fatal(err)
	}
	client := newGdpPipelineServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, gdpsdk.GuardedDataPipelineClient{BaseClient: session.BaseClient()})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*gdpv1beta1.GdpPipeline]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *gdpv1beta1.GdpPipeline) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *gdpv1beta1.GdpPipeline) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created GdpPipeline status = %+v", current.Status)
		}
		return nil
	}})
}
func mustGdpPipelineJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
