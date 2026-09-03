/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package tenancyattachment

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	resourceanalyticssdk "github.com/oracle/oci-go-sdk/v65/resourceanalytics"
	resourceanalyticsv1beta1 "github.com/oracle/oci-service-operator/api/resourceanalytics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticTenancyAttachmentCreateReadDelete(t *testing.T) {
	resource := newTenancyAttachmentTestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.tenancyattachment.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "resourceanalytics", Resource: "TenancyAttachment",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "tenancyattachment_synthetic_crud.yaml"), Host: "https://resource-analytics.us-ashburn-1.oci.oraclecloud.com", BasePath: "20241031", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := resourceanalyticssdk.TenancyAttachmentClient{BaseClient: session.BaseClient()}
	client := newTenancyAttachmentServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*resourceanalyticsv1beta1.TenancyAttachment]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *resourceanalyticsv1beta1.TenancyAttachment) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *resourceanalyticsv1beta1.TenancyAttachment) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created TenancyAttachment status = %+v", current.Status)
			}
			return nil
		},
	})
}
