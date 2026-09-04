/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package performancetuninganalysis

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	jmsutilssdk "github.com/oracle/oci-go-sdk/v65/jmsutils"
	jmsutilsv1beta1 "github.com/oracle/oci-service-operator/api/jmsutils/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticPerformanceTuningAnalysisCreateReadDelete(t *testing.T) {
	resource := newPerformanceTuningAnalysisResource()
	resourceID := "ocid1.performancetuninganalysis.oc1..synthetic"
	createdBody, err := ocireplay.SyntheticJSONBody(newPerformanceTuningAnalysis(resourceID, testWorkRequest))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(newPerformanceTuningAnalysisWorkRequest(jmsutilssdk.OperationStatusSucceeded, resourceID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "jmsutils", Resource: "PerformanceTuningAnalysis", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "performancetuninganalysis_synthetic_crud.yaml"), Host: "https://javamanagement-utils.us-ashburn-1.oci.oraclecloud.com", BasePath: "20250521", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20250521/performanceTuningAnalysis", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, CreateWorkRequestID: testWorkRequest, DeleteStatus: 204, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := jmsutilssdk.JmsUtilsClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newPerformanceTuningAnalysisServiceClientWithOCIClient(sdkClient, log)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*jmsutilsv1beta1.PerformanceTuningAnalysis]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *jmsutilsv1beta1.PerformanceTuningAnalysis) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *jmsutilsv1beta1.PerformanceTuningAnalysis) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created PerformanceTuningAnalysis status = %+v", current.Status)
		}
		return nil
	}})
}
