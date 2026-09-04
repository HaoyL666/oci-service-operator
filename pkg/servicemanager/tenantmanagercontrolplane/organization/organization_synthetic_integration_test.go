/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package organization

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

func TestSyntheticOrganizationReadsExisting(t *testing.T) {
	resource := newOrganizationResource()
	createdBody, err := ocireplay.SyntheticJSONBody(activeOrganizationSDK(testOrganizationID, testOldSubscriptionID))
	if err != nil {
		t.Fatal(err)
	}
	presentSummary, err := ocireplay.SyntheticJSONBody(organizationSummary(testOrganizationID, testOldSubscriptionID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "tenantmanagercontrolplane", Resource: "Organization", Operations: []ocireplay.Operation{ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "organization_synthetic_crud.yaml"), Host: "https://organizations.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230401", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CollectionPath: "/20230401/organizations", InitiallyPresent: true, CreatedBody: createdBody, PresentCollectionBody: "{\"items\":[" + presentSummary + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	organizationClient := tenantmanagercontrolplanesdk.OrganizationClient{BaseClient: session.BaseClient()}
	workClient := tenantmanagercontrolplanesdk.WorkRequestClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newOrganizationServiceClientWithClients(log, organizationClient, workClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*tenantmanagercontrolplanev1beta1.Organization]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *tenantmanagercontrolplanev1beta1.Organization) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *tenantmanagercontrolplanev1beta1.Organization) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created Organization status = %+v", current.Status)
		}
		return nil
	}})
}
