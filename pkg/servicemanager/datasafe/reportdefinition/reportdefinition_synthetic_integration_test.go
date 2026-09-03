/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package reportdefinition

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	datasafesdk "github.com/oracle/oci-go-sdk/v65/datasafe"
	datasafev1beta1 "github.com/oracle/oci-service-operator/api/datasafe/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticReportDefinitionCreateReadDelete(t *testing.T) {
	resource := makeReportDefinitionResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.datasafereportdefinition.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "datasafe", Resource: "ReportDefinition", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "reportdefinition_synthetic_crud.yaml"), Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	client := newReportDefinitionServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.ReportDefinition]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *datasafev1beta1.ReportDefinition) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *datasafev1beta1.ReportDefinition) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created ReportDefinition status = %+v", current.Status)
		}
		return nil
	}})
}
