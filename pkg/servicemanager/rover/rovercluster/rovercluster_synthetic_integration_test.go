/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package rovercluster

import (
	"context"
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

const syntheticRoverClusterName = "osok-replay-synthetic-rover-v1"

// A Rover Cluster represents an order for physical appliances. The synthetic
// cassette exercises the SDK and runtime contract without placing an order or
// claiming that this lifecycle was observed in a live tenancy.
func TestSyntheticRoverClusterCreateUpdateDelete(t *testing.T) {
	sdkClient, closeSession := openSyntheticRoverClusterSDK(t)
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := closeSession(); err != nil {
				t.Errorf("close RoverCluster cassette: %v", err)
			}
		}
	})

	resource := &roverv1beta1.RoverCluster{
		Spec: roverv1beta1.RoverClusterSpec{
			DisplayName:   syntheticRoverClusterName,
			CompartmentId: "ocid1.compartment.oc1..replay",
			ClusterSize:   5,
			ClusterType:   "STANDALONE",
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	manager := newSyntheticRoverClusterManager(sdkClient)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitSyntheticRoverClusterConvergence(createCtx, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(roversdk.LifecycleStateActive) ||
		resource.Status.DisplayName != syntheticRoverClusterName {
		t.Fatalf("created RoverCluster status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = syntheticRoverClusterName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitSyntheticRoverClusterConvergence(ctx, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated RoverCluster status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, ocireplay.ModeReplay, 0, func() (bool, error) {
		return manager.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func newSyntheticRoverClusterManager(
	sdkClient roversdk.RoverClusterClient,
) *RoverClusterServiceManager {
	manager := &RoverClusterServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")},
	}
	hooks := newRoverClusterRuntimeHooks(manager, sdkClient)
	client := defaultRoverClusterServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*roverv1beta1.RoverCluster](
			buildRoverClusterGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return manager.WithClient(client)
}

func openSyntheticRoverClusterSDK(t *testing.T) (roversdk.RoverClusterClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "rover",
		Resource: "RoverCluster",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: filepath.Join(
			"testdata",
			"recordings",
			"rovercluster_synthetic_crud.yaml",
		),
		Host:     "https://rover.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20201210",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return roversdk.RoverClusterClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitSyntheticRoverClusterConvergence(
	ctx context.Context,
	manager *RoverClusterServiceManager,
	resource *roverv1beta1.RoverCluster,
) error {
	return ocireplay.Await(ctx, ocireplay.ModeReplay, 0, func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("RoverCluster reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}
