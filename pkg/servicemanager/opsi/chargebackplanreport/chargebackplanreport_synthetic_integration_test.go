/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package chargebackplanreport

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	opsisdk "github.com/oracle/oci-go-sdk/v65/opsi"
	opsiv1beta1 "github.com/oracle/oci-service-operator/api/opsi/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticChargebackPlanReportCreateReadDelete(t *testing.T) {
	resource := newChargebackPlanReportResource()
	createdBody, err := ocireplay.SyntheticJSONBody(newSDKChargebackPlanReport(testReportID, testReportName, testSourceID, testResourceType, testTimeEnd))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(newChargebackPlanReportWorkRequest("ocid1.workrequest.oc1..syntheticcreate", opsisdk.OperationStatusSucceeded, opsisdk.ActionTypeCreated, testReportID))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(newChargebackPlanReportWorkRequest("ocid1.workrequest.oc1..syntheticdelete", opsisdk.OperationStatusSucceeded, opsisdk.ActionTypeDeleted, testReportID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "opsi", Resource: "ChargebackPlanReport", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "chargebackplanreport_synthetic_crud.yaml"), Host: "https://operationsinsights.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200630", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20200630/chargebackPlanReports", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := opsisdk.OperationsInsightsClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	hooks := newChargebackPlanReportDefaultRuntimeHooks(sdkClient)
	configureChargebackPlanReportRuntimeHooks(&hooks, sdkClient, nil, log)
	client := wrapChargebackPlanReportGeneratedClient(hooks, defaultChargebackPlanReportServiceClient{ServiceClient: generatedruntime.NewServiceClient[*opsiv1beta1.ChargebackPlanReport](buildChargebackPlanReportGeneratedRuntimeConfig(&ChargebackPlanReportServiceManager{Log: log}, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*opsiv1beta1.ChargebackPlanReport]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *opsiv1beta1.ChargebackPlanReport) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *opsiv1beta1.ChargebackPlanReport) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created ChargebackPlanReport status = %+v", current.Status)
		}
		return nil
	}})
}
