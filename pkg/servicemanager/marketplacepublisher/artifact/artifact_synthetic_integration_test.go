/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package artifact

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	marketplacepublishersdk "github.com/oracle/oci-go-sdk/v65/marketplacepublisher"
	marketplacepublisherv1beta1 "github.com/oracle/oci-service-operator/api/marketplacepublisher/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticArtifactCreateReadDelete(t *testing.T) {
	resource := newArtifactResource()
	createdBody, err := ocireplay.SyntheticJSONBody(sdkArtifactFromSpec(testArtifactID, resource.Spec, marketplacepublishersdk.ArtifactLifecycleStateActive))
	if err != nil {
		t.Fatal(err)
	}
	createWorkRequest, err := ocireplay.SyntheticJSONBody(artifactWorkRequest("ocid1.workrequest.oc1..syntheticcreate", marketplacepublishersdk.OperationTypeCreateArtifact, marketplacepublishersdk.OperationStatusSucceeded, marketplacepublishersdk.ActionTypeCreated, testArtifactID))
	if err != nil {
		t.Fatal(err)
	}
	deleteWorkRequest, err := ocireplay.SyntheticJSONBody(artifactWorkRequest("ocid1.workrequest.oc1..syntheticdelete", marketplacepublishersdk.OperationTypeDeleteArtifact, marketplacepublishersdk.OperationStatusSucceeded, marketplacepublishersdk.ActionTypeDeleted, testArtifactID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "marketplacepublisher", Resource: "Artifact", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "artifact_synthetic_crud.yaml"), Host: "https://marketplace-publisher.us-ashburn-1.oci.oraclecloud.com", BasePath: "20241201", Metadata: metadata, Responder: ocireplay.NewSyntheticWorkRequestCRUDResponder(ocireplay.SyntheticWorkRequestCRUDOptions{CreatedBody: createdBody, CreateWorkRequestBody: createWorkRequest, DeleteWorkRequestBody: deleteWorkRequest, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := marketplacepublishersdk.MarketplacePublisherClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	client := newArtifactServiceClientWithOCIClient(log, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*marketplacepublisherv1beta1.Artifact]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *marketplacepublisherv1beta1.Artifact) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *marketplacepublisherv1beta1.Artifact) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created Artifact status = %+v", current.Status)
		}
		return nil
	}})
}
