/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package databasetoolsconnection

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	databasetoolssdk "github.com/oracle/oci-go-sdk/v65/databasetools"
	databasetoolsv1beta1 "github.com/oracle/oci-service-operator/api/databasetools/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDatabaseToolsConnectionCreateReadDelete(t *testing.T) {
	resource := makeGenericJDBCResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.databasetoolsconnection.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "databasetools", Resource: "DatabaseToolsConnection", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "databasetoolsconnection_synthetic_crud.yaml"), Host: "https://dbtools.us-ashburn-1.oci.oraclecloud.com", BasePath: "20201005", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := databasetoolssdk.DatabaseToolsClient{BaseClient: session.BaseClient()}
	client := newDatabaseToolsConnectionServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, nil, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*databasetoolsv1beta1.DatabaseToolsConnection]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *databasetoolsv1beta1.DatabaseToolsConnection) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *databasetoolsv1beta1.DatabaseToolsConnection) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created DatabaseToolsConnection status = %+v", current.Status)
		}
		return nil
	}})
}
