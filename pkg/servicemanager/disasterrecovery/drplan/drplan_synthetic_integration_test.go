/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package drplan

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	disasterrecoverysdk "github.com/oracle/oci-go-sdk/v65/disasterrecovery"
	disasterrecoveryv1beta1 "github.com/oracle/oci-service-operator/api/disasterrecovery/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDrPlanCreateReadDelete(t *testing.T) {
	resource := newDrPlanTestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.drplan.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticWorkRequestBody("ocid1.workrequest.oc1..syntheticcreate", string(disasterrecoverysdk.OperationTypeCreateDrPlan), string(disasterrecoverysdk.OperationStatusSucceeded), string(disasterrecoverysdk.ActionTypeCreated), "DrPlan", "ocid1.drplan.oc1..synthetic")
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "disasterrecovery", Resource: "DrPlan", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "drplan_synthetic_crud.yaml"), Host: "https://disaster-recovery.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220125", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20220125/drPlans", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteStatus: 204, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := disasterrecoverysdk.DisasterRecoveryClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	manager := &DrPlanServiceManager{Log: log}
	hooks := newDrPlanDefaultRuntimeHooks(sdkClient)
	applyDrPlanRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapDrPlanGeneratedClient(hooks, defaultDrPlanServiceClient{ServiceClient: generatedruntime.NewServiceClient[*disasterrecoveryv1beta1.DrPlan](buildDrPlanGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*disasterrecoveryv1beta1.DrPlan]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *disasterrecoveryv1beta1.DrPlan) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *disasterrecoveryv1beta1.DrPlan) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created DrPlan status = %+v", current.Status)
		}
		return nil
	}})
}
