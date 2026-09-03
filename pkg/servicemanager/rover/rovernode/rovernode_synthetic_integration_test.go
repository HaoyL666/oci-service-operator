/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package rovernode

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	roversdk "github.com/oracle/oci-go-sdk/v65/rover"
	roverv1beta1 "github.com/oracle/oci-service-operator/api/rover/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticRoverNodeCreateReadDelete(t *testing.T) {
	resource := &roverv1beta1.RoverNode{Spec: roverv1beta1.RoverNodeSpec{
		DisplayName: "osok-replay-rover-node", CompartmentId: "ocid1.compartment.oc1..replay", Shape: "Rover.Node.1.168",
	}}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.rovernode.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "rover", Resource: "RoverNode", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "rovernode_synthetic_crud.yaml"), Host: "https://rover.us-ashburn-1.oci.oraclecloud.com", BasePath: "20201210", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := roversdk.RoverNodeClient{BaseClient: session.BaseClient()}
	manager := &RoverNodeServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newRoverNodeRuntimeHooks(manager, sdkClient)
	client := wrapRoverNodeGeneratedClient(hooks, defaultRoverNodeServiceClient{ServiceClient: generatedruntime.NewServiceClient[*roverv1beta1.RoverNode](buildRoverNodeGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*roverv1beta1.RoverNode]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *roverv1beta1.RoverNode) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *roverv1beta1.RoverNode) error {
			if current.Status.OsokStatus.Ocid == "" || current.Status.DisplayName != resource.Spec.DisplayName {
				return fmt.Errorf("created RoverNode status = %+v", current.Status)
			}
			return nil
		},
	})
}
