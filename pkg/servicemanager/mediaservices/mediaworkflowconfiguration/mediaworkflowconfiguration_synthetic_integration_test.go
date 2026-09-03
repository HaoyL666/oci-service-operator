/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package mediaworkflowconfiguration

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	mediaservicessdk "github.com/oracle/oci-go-sdk/v65/mediaservices"
	mediaservicesv1beta1 "github.com/oracle/oci-service-operator/api/mediaservices/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticMediaWorkflowConfigurationCreateReadDelete(t *testing.T) {
	resource := newMediaWorkflowConfigurationTestResource()
	resource.Spec.Locks = nil
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.mediaworkflowconfiguration.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "mediaservices", Resource: "MediaWorkflowConfiguration",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "mediaworkflowconfiguration_synthetic_crud.yaml"), Host: "https://mediaservices.us-ashburn-1.oci.oraclecloud.com", BasePath: "20211101", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := mediaservicessdk.MediaServicesClient{BaseClient: session.BaseClient()}
	client := newMediaWorkflowConfigurationServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*mediaservicesv1beta1.MediaWorkflowConfiguration]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *mediaservicesv1beta1.MediaWorkflowConfiguration) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *mediaservicesv1beta1.MediaWorkflowConfiguration) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created MediaWorkflowConfiguration status = %+v", current.Status)
			}
			return nil
		},
	})
}
