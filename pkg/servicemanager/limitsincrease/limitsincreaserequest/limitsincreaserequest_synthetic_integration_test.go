/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package limitsincreaserequest

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	limitsincreasesdk "github.com/oracle/oci-go-sdk/v65/limitsincrease"
	limitsincreasev1beta1 "github.com/oracle/oci-service-operator/api/limitsincrease/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestSyntheticLimitsIncreaseRequestCreateReadDelete(t *testing.T) {
	resource := makeLimitsIncreaseRequestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.limitsincreaserequest.oc1..synthetic", "SUCCEEDED", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "limitsincrease", Resource: "LimitsIncreaseRequest", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "limitsincreaserequest_synthetic_crud.yaml"), Host: "https://limitsincrease.us-ashburn-1.oci.oraclecloud.com", BasePath: "20251101", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := limitsincreasesdk.LimitsIncreaseClient{BaseClient: session.BaseClient()}
	client := testLimitsIncreaseRequestClient(&fakeLimitsIncreaseRequestOCIClient{createFn: sdkClient.CreateLimitsIncreaseRequest, getFn: sdkClient.GetLimitsIncreaseRequest, listFn: sdkClient.ListLimitsIncreaseRequests, updateFn: sdkClient.UpdateLimitsIncreaseRequest, deleteFn: sdkClient.DeleteLimitsIncreaseRequest})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*limitsincreasev1beta1.LimitsIncreaseRequest]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *limitsincreasev1beta1.LimitsIncreaseRequest) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *limitsincreasev1beta1.LimitsIncreaseRequest) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created LimitsIncreaseRequest status = %+v", current.Status)
		}
		return nil
	}})
}
