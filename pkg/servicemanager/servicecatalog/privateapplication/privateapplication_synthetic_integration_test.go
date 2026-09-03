/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package privateapplication

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	servicecatalogsdk "github.com/oracle/oci-go-sdk/v65/servicecatalog"
	servicecatalogv1beta1 "github.com/oracle/oci-service-operator/api/servicecatalog/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticPrivateApplicationCreateReadDelete(t *testing.T) {
	resource := makePrivateApplicationResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.privateapplication.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "servicecatalog", Resource: "PrivateApplication",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "privateapplication_synthetic_crud.yaml"), Host: "https://service-catalog.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210527", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := servicecatalogsdk.ServiceCatalogClient{BaseClient: session.BaseClient()}
	client := newPrivateApplicationServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*servicecatalogv1beta1.PrivateApplication]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *servicecatalogv1beta1.PrivateApplication) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *servicecatalogv1beta1.PrivateApplication) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created PrivateApplication status = %+v", current.Status)
			}
			return nil
		},
	})
}
