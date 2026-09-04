/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package drprotectiongroup

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

func TestSyntheticDrProtectionGroupCreateReadDelete(t *testing.T) {
	resource := newDrProtectionGroupTestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.drprotectiongroup.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticWorkRequestBody("ocid1.workrequest.oc1..syntheticcreate", string(disasterrecoverysdk.OperationTypeCreateDrProtectionGroup), string(disasterrecoverysdk.OperationStatusSucceeded), string(disasterrecoverysdk.ActionTypeCreated), "DrProtectionGroup", "ocid1.drprotectiongroup.oc1..synthetic")
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticWorkRequestBody("ocid1.workrequest.oc1..syntheticdelete", string(disasterrecoverysdk.OperationTypeDeleteDrProtectionGroup), string(disasterrecoverysdk.OperationStatusSucceeded), string(disasterrecoverysdk.ActionTypeDeleted), "DrProtectionGroup", "ocid1.drprotectiongroup.oc1..synthetic")
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "disasterrecovery", Resource: "DrProtectionGroup", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "drprotectiongroup_synthetic_crud.yaml"), Host: "https://disaster-recovery.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220125", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20220125/drProtectionGroups", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := disasterrecoverysdk.DisasterRecoveryClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	manager := &DrProtectionGroupServiceManager{Log: log}
	hooks := newDrProtectionGroupDefaultRuntimeHooks(sdkClient)
	applyDrProtectionGroupRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapDrProtectionGroupGeneratedClient(hooks, defaultDrProtectionGroupServiceClient{ServiceClient: generatedruntime.NewServiceClient[*disasterrecoveryv1beta1.DrProtectionGroup](buildDrProtectionGroupGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*disasterrecoveryv1beta1.DrProtectionGroup]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *disasterrecoveryv1beta1.DrProtectionGroup) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *disasterrecoveryv1beta1.DrProtectionGroup) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created DrProtectionGroup status = %+v", current.Status)
		}
		return nil
	}})
}
