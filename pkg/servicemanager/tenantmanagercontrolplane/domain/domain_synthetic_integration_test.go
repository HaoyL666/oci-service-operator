/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package domain

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	tenantmanagercontrolplanesdk "github.com/oracle/oci-go-sdk/v65/tenantmanagercontrolplane"
	tenantmanagercontrolplanev1beta1 "github.com/oracle/oci-service-operator/api/tenantmanagercontrolplane/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDomainCreateReadDelete(t *testing.T) {
	resource := newDomainResource()
	createdBody, err := ocireplay.SyntheticJSONBody(activeDomainSDK(testDomainID, tenantmanagercontrolplanesdk.DomainStatusActive, resource.Spec.FreeformTags))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(domainWorkRequest("ocid1.workrequest.oc1..syntheticcreate", tenantmanagercontrolplanesdk.OperationStatusSucceeded, tenantmanagercontrolplanesdk.ActionTypeCreated, testDomainID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "tenantmanagercontrolplane", Resource: "Domain", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "domain_synthetic_crud.yaml"), Host: "https://organizations.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230401", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CollectionPath: "/20230401/domains", CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteStatus: 204, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	domainClient := tenantmanagercontrolplanesdk.DomainClient{BaseClient: session.BaseClient()}
	workClient := tenantmanagercontrolplanesdk.WorkRequestClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newDomainServiceClientWithClients(log, domainClient, workClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*tenantmanagercontrolplanev1beta1.Domain]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *tenantmanagercontrolplanev1beta1.Domain) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *tenantmanagercontrolplanev1beta1.Domain) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created Domain status = %+v", current.Status)
		}
		return nil
	}})
}
