/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package opensearchcluster

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	opensearchsdk "github.com/oracle/oci-go-sdk/v65/opensearch"
	opensearchv1beta1 "github.com/oracle/oci-service-operator/api/opensearch/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const syntheticOpensearchClusterName = "osok-replay-opensearch-v1"

// This topology allocates three master nodes, three data nodes with block
// storage, and one dashboard node. The synthetic cassette exercises the OCI SDK
// and runtime contract without creating that paid compute/storage footprint or
// claiming the lifecycle was observed in a live tenancy.
func TestSyntheticOpensearchClusterCreateUpdateDelete(t *testing.T) {
	sdkClient, closeSession := openSyntheticOpensearchClusterSDK(t)
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := closeSession(); err != nil {
				t.Errorf("close OpensearchCluster cassette: %v", err)
			}
		}
	})

	resource := &opensearchv1beta1.OpensearchCluster{Spec: opensearchv1beta1.OpensearchClusterSpec{
		DisplayName:                    syntheticOpensearchClusterName,
		CompartmentId:                  "ocid1.compartment.oc1..replay",
		SoftwareVersion:                "2.11.0",
		MasterNodeCount:                3,
		MasterNodeHostType:             string(opensearchsdk.MasterNodeHostTypeFlex),
		MasterNodeHostOcpuCount:        1,
		MasterNodeHostMemoryGB:         16,
		DataNodeCount:                  3,
		DataNodeHostType:               string(opensearchsdk.DataNodeHostTypeFlex),
		DataNodeHostOcpuCount:          2,
		DataNodeHostMemoryGB:           32,
		DataNodeStorageGB:              50,
		OpendashboardNodeCount:         1,
		OpendashboardNodeHostOcpuCount: 1,
		OpendashboardNodeHostMemoryGB:  8,
		VcnId:                          "ocid1.vcn.oc1..replay",
		SubnetId:                       "ocid1.subnet.oc1..replay",
		VcnCompartmentId:               "ocid1.compartment.oc1..replay",
		SubnetCompartmentId:            "ocid1.compartment.oc1..replay",
		SecurityMode:                   string(opensearchsdk.SecurityModeDisabled),
		FreeformTags:                   map[string]string{"osok-replay": "synthetic"},
	}}
	client := newSyntheticOpensearchClusterClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitSyntheticOpensearchClusterConvergence(createCtx, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(opensearchsdk.OpensearchClusterLifecycleStateActive) ||
		resource.Status.DisplayName != syntheticOpensearchClusterName ||
		resource.Status.DataNodeCount != 3 {
		t.Fatalf("created OpensearchCluster status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = syntheticOpensearchClusterName + "-updated"
	if err := awaitSyntheticOpensearchClusterConvergence(ctx, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName {
		t.Fatalf("updated OpensearchCluster status = %+v", resource.Status)
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

func newSyntheticOpensearchClusterClient(
	sdkClient opensearchsdk.OpensearchClusterClient,
) OpensearchClusterServiceClient {
	manager := &OpensearchClusterServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")},
	}
	hooks := newOpensearchClusterDefaultRuntimeHooks(sdkClient)
	applyOpensearchClusterRuntimeHooks(manager, &hooks)
	delegate := defaultOpensearchClusterServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*opensearchv1beta1.OpensearchCluster](
			buildOpensearchClusterGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return wrapOpensearchClusterGeneratedClient(hooks, delegate)
}

func openSyntheticOpensearchClusterSDK(
	t *testing.T,
) (opensearchsdk.OpensearchClusterClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "opensearch",
		Resource: "OpensearchCluster",
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
			"opensearchcluster_synthetic_crud.yaml",
		),
		Host:     "https://opensearch.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20180828",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return opensearchsdk.OpensearchClusterClient{
		BaseClient: session.BaseClient(),
	}, session.Close
}

func awaitSyntheticOpensearchClusterConvergence(
	ctx context.Context,
	client OpensearchClusterServiceClient,
	resource *opensearchv1beta1.OpensearchCluster,
) error {
	return ocireplay.Await(ctx, ocireplay.ModeReplay, 0, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf(
				"OpensearchCluster reconciliation was unsuccessful: %+v",
				response,
			)
		}
		return !response.ShouldRequeue, nil
	})
}
