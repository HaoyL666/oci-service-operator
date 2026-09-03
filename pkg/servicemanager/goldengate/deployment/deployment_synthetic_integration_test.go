/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package deployment

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	goldengatesdk "github.com/oracle/oci-go-sdk/v65/goldengate"
	goldengatev1beta1 "github.com/oracle/oci-service-operator/api/goldengate/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDeploymentCreateReadDelete(t *testing.T) {
	resource := &goldengatev1beta1.Deployment{Spec: goldengatev1beta1.DeploymentSpec{
		DisplayName: "osok-replay-goldengate", CompartmentId: "ocid1.compartment.oc1..replay", SubnetId: "ocid1.subnet.oc1..replay",
		FreeformTags: map[string]string{"osok-replay": "synthetic"},
	}}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.goldengatedeployment.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "goldengate", Resource: "Deployment", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "deployment_synthetic_crud.yaml"), Host: "https://goldengate.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200407", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := goldengatesdk.GoldenGateClient{BaseClient: session.BaseClient()}
	manager := &DeploymentServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newDeploymentRuntimeHooks(manager, sdkClient)
	client := wrapDeploymentGeneratedClient(hooks, defaultDeploymentServiceClient{ServiceClient: generatedruntime.NewServiceClient[*goldengatev1beta1.Deployment](buildDeploymentGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*goldengatev1beta1.Deployment]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *goldengatev1beta1.Deployment) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *goldengatev1beta1.Deployment) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.DisplayName != resource.Spec.DisplayName {
				return fmt.Errorf("created Deployment status = %+v", current.Status)
			}
			return nil
		},
	})
}
