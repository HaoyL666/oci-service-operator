/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package occcapacityrequest

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	capacitymanagementsdk "github.com/oracle/oci-go-sdk/v65/capacitymanagement"
	capacitymanagementv1beta1 "github.com/oracle/oci-service-operator/api/capacitymanagement/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticOccCapacityRequestCreateReadDelete(t *testing.T) {
	resource := newOccCapacityRequestTestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.occcapacityrequest.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "capacitymanagement", Resource: "OccCapacityRequest", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "occcapacityrequest_synthetic_crud.yaml"), Host: "https://capacity-management.us-ashburn-1.oci.oraclecloud.com", BasePath: "20231107", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := capacitymanagementsdk.CapacityManagementClient{BaseClient: session.BaseClient()}
	client := newOccCapacityRequestServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*capacitymanagementv1beta1.OccCapacityRequest]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *capacitymanagementv1beta1.OccCapacityRequest) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *capacitymanagementv1beta1.OccCapacityRequest) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created OccCapacityRequest status = %+v", current.Status)
		}
		return nil
	}})
}
