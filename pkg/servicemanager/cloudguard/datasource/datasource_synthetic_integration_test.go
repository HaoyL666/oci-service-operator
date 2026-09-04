/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package datasource

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	cloudguardsdk "github.com/oracle/oci-go-sdk/v65/cloudguard"
	cloudguardv1beta1 "github.com/oracle/oci-service-operator/api/cloudguard/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDataSourceCreateReadDelete(t *testing.T) {
	resource := newDataSourceRuntimeTestResource()
	createdBody, err := ocireplay.SyntheticJSONBody(dataSourceFromSpec(t, testDataSourceID, resource.Spec, cloudguardsdk.LifecycleStateActive))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(dataSourceWorkRequest("ocid1.workrequest.oc1..syntheticcreate", cloudguardsdk.OperationTypeCreate, cloudguardsdk.OperationStatusSucceeded, testDataSourceID))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(dataSourceWorkRequest("ocid1.workrequest.oc1..syntheticdelete", cloudguardsdk.OperationTypeDelete, cloudguardsdk.OperationStatusSucceeded, testDataSourceID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "cloudguard", Resource: "DataSource", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "datasource_synthetic_crud.yaml"), Host: "https://cloudguard-cp-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200131", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20200131/dataSources", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := cloudguardsdk.CloudGuardClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	manager := &DataSourceServiceManager{Log: log}
	hooks := newDataSourceRuntimeHooksWithOCIClient(sdkClient)
	applyDataSourceRuntimeHooksWithWorkRequestClient(manager, &hooks, sdkClient, nil)
	client := wrapDataSourceGeneratedClient(hooks, defaultDataSourceServiceClient{ServiceClient: generatedruntime.NewServiceClient[*cloudguardv1beta1.DataSource](buildDataSourceGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*cloudguardv1beta1.DataSource]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *cloudguardv1beta1.DataSource) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *cloudguardv1beta1.DataSource) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created DataSource status = %+v", current.Status)
		}
		return nil
	}})
}
