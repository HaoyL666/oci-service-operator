/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package domaingovernance

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

func TestSyntheticDomainGovernanceCreateReadDelete(t *testing.T) {
	resource := newDomainGovernanceResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.domaingovernance.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "tenantmanagercontrolplane", Resource: "DomainGovernance", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "domaingovernance_synthetic_crud.yaml"), Host: "https://tenant-manager-control-plane.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230401", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := tenantmanagercontrolplanesdk.DomainGovernanceClient{BaseClient: session.BaseClient()}
	client := newDomainGovernanceServiceClientWithClients(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*tenantmanagercontrolplanev1beta1.DomainGovernance]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *tenantmanagercontrolplanev1beta1.DomainGovernance) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *tenantmanagercontrolplanev1beta1.DomainGovernance) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created DomainGovernance status = %+v", current.Status)
		}
		return nil
	}})
}
