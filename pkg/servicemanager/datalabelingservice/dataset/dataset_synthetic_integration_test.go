/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dataset

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	datalabelingservicesdk "github.com/oracle/oci-go-sdk/v65/datalabelingservice"
	datalabelingservicev1beta1 "github.com/oracle/oci-service-operator/api/datalabelingservice/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDatasetCreateReadDelete(t *testing.T) {
	resource := newDatasetTestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.datalabelingdataset.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "datalabelingservice", Resource: "Dataset", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "dataset_synthetic_crud.yaml"), Host: "https://datalabeling-cp.us-ashburn-1.oci.oraclecloud.com", BasePath: "20211001", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datalabelingservicesdk.DataLabelingManagementClient{BaseClient: session.BaseClient()}
	client := newDatasetServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datalabelingservicev1beta1.Dataset]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *datalabelingservicev1beta1.Dataset) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *datalabelingservicev1beta1.Dataset) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created Dataset status = %+v", current.Status)
		}
		return nil
	}})
}
