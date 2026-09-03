/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package profile

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	optimizersdk "github.com/oracle/oci-go-sdk/v65/optimizer"
	optimizerv1beta1 "github.com/oracle/oci-service-operator/api/optimizer/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticProfileCreateReadDelete(t *testing.T) {
	resource := newProfileRuntimeTestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.optimizerprofile.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "optimizer", Resource: "Profile",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "profile_synthetic_crud.yaml"), Host: "https://optimizer.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200606", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := optimizersdk.OptimizerClient{BaseClient: session.BaseClient()}
	manager := &ProfileServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newProfileDefaultRuntimeHooks(sdkClient)
	applyProfileRuntimeHooks(&hooks)
	client := wrapProfileGeneratedClient(hooks, defaultProfileServiceClient{ServiceClient: generatedruntime.NewServiceClient[*optimizerv1beta1.Profile](buildProfileGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*optimizerv1beta1.Profile]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *optimizerv1beta1.Profile) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *optimizerv1beta1.Profile) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created Profile status = %+v", current.Status)
			}
			return nil
		},
	})
}
