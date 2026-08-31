/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package dedicatedaicluster

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	generativeaisdk "github.com/oracle/oci-go-sdk/v65/generativeai"
	generativeaiv1beta1 "github.com/oracle/oci-service-operator/api/generativeai/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const syntheticDedicatedAiClusterName = "osok-replay-synthetic-ai-cluster-v1"

// Dedicated AI clusters consume scarce paid accelerator capacity. This test
// exercises the published SDK and runtime contract without allocating units or
// claiming that the lifecycle was observed in a live tenancy.
func TestSyntheticDedicatedAiClusterCreateUpdateDelete(t *testing.T) {
	sdkClient, closeSession := openSyntheticDedicatedAiClusterSDK(t)
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := closeSession(); err != nil {
				t.Errorf("close DedicatedAiCluster cassette: %v", err)
			}
		}
	})

	resource := &generativeaiv1beta1.DedicatedAiCluster{
		Spec: generativeaiv1beta1.DedicatedAiClusterSpec{
			Type:          string(generativeaisdk.DedicatedAiClusterTypeHosting),
			CompartmentId: "ocid1.compartment.oc1..replay",
			UnitCount:     1,
			UnitShape: string(
				generativeaisdk.DedicatedAiClusterUnitShapeSmallCohere,
			),
			DisplayName: syntheticDedicatedAiClusterName,
			Description: "synthetic create",
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	client := newSyntheticDedicatedAiClusterClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitSyntheticDedicatedAiClusterConvergence(createCtx, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(
		generativeaisdk.DedicatedAiClusterLifecycleStateActive,
	) || resource.Status.UnitCount != 1 {
		t.Fatalf("created DedicatedAiCluster status = %+v", resource.Status)
	}

	resource.Spec.UnitCount = 2
	resource.Spec.Description = "synthetic update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitSyntheticDedicatedAiClusterConvergence(ctx, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.UnitCount != 2 ||
		resource.Status.Description != resource.Spec.Description ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated DedicatedAiCluster status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, ocireplay.ModeReplay, 0, func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func newSyntheticDedicatedAiClusterClient(
	sdkClient generativeaisdk.GenerativeAiClient,
) DedicatedAiClusterServiceClient {
	manager := &DedicatedAiClusterServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")},
	}
	hooks := newDedicatedAiClusterDefaultRuntimeHooks(sdkClient)
	applyDedicatedAiClusterRuntimeHooks(&hooks)
	delegate := defaultDedicatedAiClusterServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*generativeaiv1beta1.DedicatedAiCluster](
			buildDedicatedAiClusterGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return wrapDedicatedAiClusterGeneratedClient(hooks, delegate)
}

func openSyntheticDedicatedAiClusterSDK(
	t *testing.T,
) (generativeaisdk.GenerativeAiClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "generativeai",
		Resource: "DedicatedAiCluster",
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
			"dedicatedaicluster_synthetic_crud.yaml",
		),
		Host:     "https://generativeai.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20231130",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return generativeaisdk.GenerativeAiClient{
		BaseClient: session.BaseClient(),
	}, session.Close
}

func awaitSyntheticDedicatedAiClusterConvergence(
	ctx context.Context,
	client DedicatedAiClusterServiceClient,
	resource *generativeaiv1beta1.DedicatedAiCluster,
) error {
	return ocireplay.Await(ctx, ocireplay.ModeReplay, 0, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf(
				"DedicatedAiCluster reconciliation was unsuccessful: %+v",
				response,
			)
		}
		return !response.ShouldRequeue, nil
	})
}
