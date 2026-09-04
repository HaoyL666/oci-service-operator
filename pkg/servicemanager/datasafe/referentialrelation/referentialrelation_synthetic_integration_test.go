/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package referentialrelation

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

func TestSyntheticReferentialRelationCreateReadDelete(t *testing.T) {
	resource := makeReferentialRelationResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "", "ACTIVE", map[string]any{
		"key":                  "42",
		"sensitiveDataModelId": testReferentialRelationSensitiveDataModelID,
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "datasafe", Resource: "ReferentialRelation", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "referentialrelation_synthetic_crud.yaml"), Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	client := newReferentialRelationServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.ReferentialRelation]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *datasafev1beta1.ReferentialRelation) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *datasafev1beta1.ReferentialRelation) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created ReferentialRelation status = %+v", current.Status)
		}
		return nil
	}})
}
