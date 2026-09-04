/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package governancerule

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	governancerulescontrolplanesdk "github.com/oracle/oci-go-sdk/v65/governancerulescontrolplane"
	governancerulescontrolplanev1beta1 "github.com/oracle/oci-service-operator/api/governancerulescontrolplane/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

type syntheticGovernanceRuleClient struct {
	governancerulescontrolplanesdk.GovernanceRuleClient
	governancerulescontrolplanesdk.WorkRequestClient
}

func TestSyntheticGovernanceRuleCreateReadDelete(t *testing.T) {
	resource := makeGovernanceRuleResource()
	createdBody, err := ocireplay.SyntheticJSONBody(makeSDKGovernanceRule(testGovernanceRuleID, resource.Spec.CompartmentId, resource.Spec.DisplayName, resource.Spec.Description, governancerulescontrolplanesdk.GovernanceRuleLifecycleStateActive, makeSDKQuotaTemplate(resource.Spec.Template.DisplayName, resource.Spec.Template.Description, resource.Spec.Template.Statements)))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(makeGovernanceRuleWorkRequest("ocid1.workrequest.oc1..syntheticcreate", governancerulescontrolplanesdk.OperationStatusSucceeded, governancerulescontrolplanesdk.OperationTypeCreateGovernanceRule, governancerulescontrolplanesdk.ActionTypeCreated))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(makeGovernanceRuleWorkRequest("ocid1.workrequest.oc1..syntheticdelete", governancerulescontrolplanesdk.OperationStatusSucceeded, governancerulescontrolplanesdk.OperationTypeDeleteGovernanceRule, governancerulescontrolplanesdk.ActionTypeDeleted))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "governancerulescontrolplane", Resource: "GovernanceRule", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "governancerule_synthetic_crud.yaml"), Host: "https://governance-rules.organizations.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220504", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20220504/governanceRules", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := syntheticGovernanceRuleClient{
		GovernanceRuleClient: governancerulescontrolplanesdk.GovernanceRuleClient{BaseClient: session.BaseClient()},
		WorkRequestClient:    governancerulescontrolplanesdk.WorkRequestClient{BaseClient: session.BaseClient()},
	}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	manager := &GovernanceRuleServiceManager{Log: log}
	hooks := newGovernanceRuleRuntimeHooksWithOCIClient(sdkClient)
	applyGovernanceRuleRuntimeHooks(&hooks, sdkClient, nil)
	client := wrapGovernanceRuleGeneratedClient(hooks, defaultGovernanceRuleServiceClient{ServiceClient: generatedruntime.NewServiceClient[*governancerulescontrolplanev1beta1.GovernanceRule](buildGovernanceRuleGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*governancerulescontrolplanev1beta1.GovernanceRule]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *governancerulescontrolplanev1beta1.GovernanceRule) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *governancerulescontrolplanev1beta1.GovernanceRule) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created GovernanceRule status = %+v", current.Status)
		}
		return nil
	}})
}
