/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package containerimagesignature

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	artifactssdk "github.com/oracle/oci-go-sdk/v65/artifacts"
	artifactsv1beta1 "github.com/oracle/oci-service-operator/api/artifacts/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticContainerImageSignatureCreateReadDelete(t *testing.T) {
	resource := newContainerImageSignatureResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.containerimagesignature.oc1..synthetic", "AVAILABLE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "artifacts", Resource: "ContainerImageSignature", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "containerimagesignature_synthetic_crud.yaml"), Host: "https://artifacts.us-ashburn-1.oci.oraclecloud.com", BasePath: "20160918", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := artifactssdk.ArtifactsClient{BaseClient: session.BaseClient()}
	manager := &ContainerImageSignatureServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newContainerImageSignatureDefaultRuntimeHooks(sdkClient)
	applyContainerImageSignatureRuntimeHooks(&hooks)
	client := wrapContainerImageSignatureGeneratedClient(hooks, defaultContainerImageSignatureServiceClient{ServiceClient: generatedruntime.NewServiceClient[*artifactsv1beta1.ContainerImageSignature](buildContainerImageSignatureGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*artifactsv1beta1.ContainerImageSignature]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity: func(current *artifactsv1beta1.ContainerImageSignature) bool {
			return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
		},
		ValidateCreated: func(current *artifactsv1beta1.ContainerImageSignature) error {
			if current.Status.Id == "" {
				return fmt.Errorf("created ContainerImageSignature status = %+v", current.Status)
			}
			return nil
		},
	})
}
