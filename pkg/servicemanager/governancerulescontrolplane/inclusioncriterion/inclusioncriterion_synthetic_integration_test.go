/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package inclusioncriterion

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	governancerulescontrolplanesdk "github.com/oracle/oci-go-sdk/v65/governancerulescontrolplane"
	governancerulescontrolplanev1beta1 "github.com/oracle/oci-service-operator/api/governancerulescontrolplane/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestSyntheticInclusionCriterionCreateReadDelete(t *testing.T) {
	resource := makeInclusionCriterionResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.inclusioncriterion.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "governancerulescontrolplane", Resource: "InclusionCriterion", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "inclusioncriterion_synthetic_crud.yaml"), Host: "https://governance-rules.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220504", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := governancerulescontrolplanesdk.GovernanceRuleClient{BaseClient: session.BaseClient()}
	client := testInclusionCriterionClient(&fakeInclusionCriterionOCIClient{createFn: sdkClient.CreateInclusionCriterion, getFn: sdkClient.GetInclusionCriterion, deleteFn: sdkClient.DeleteInclusionCriterion})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*governancerulescontrolplanev1beta1.InclusionCriterion]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *governancerulescontrolplanev1beta1.InclusionCriterion) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *governancerulescontrolplanev1beta1.InclusionCriterion) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created InclusionCriterion status = %+v", current.Status)
		}
		return nil
	}})
}
