/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package clusterplacementgroup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	clusterplacementgroupssdk "github.com/oracle/oci-go-sdk/v65/clusterplacementgroups"
	clusterplacementgroupsv1beta1 "github.com/oracle/oci-service-operator/api/clusterplacementgroups/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedClusterPlacementGroupName = "osok-replay-async-cpg-v1"

func TestRecordedClusterPlacementGroupCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}

	sdkClient, closeSession := openRecordedClusterPlacementGroupSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close ClusterPlacementGroup cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	availabilityDomain := "example:AD-1"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredClusterPlacementGroupRecordingEnv(t, "OCI_COMPARTMENT_ID")
		availabilityDomain = requiredClusterPlacementGroupRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN")
	}

	resource := &clusterplacementgroupsv1beta1.ClusterPlacementGroup{
		Spec: clusterplacementgroupsv1beta1.ClusterPlacementGroupSpec{
			Name:                      recordedClusterPlacementGroupName,
			ClusterPlacementGroupType: "STANDARD",
			Description:               "recorded create",
			AvailabilityDomain:        availabilityDomain,
			CompartmentId:             compartmentID,
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	client := newClusterPlacementGroupServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 5*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitClusterPlacementGroupConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(clusterplacementgroupssdk.ClusterPlacementGroupLifecycleStateActive) ||
		resource.Status.Name != recordedClusterPlacementGroupName {
		t.Fatalf("created ClusterPlacementGroup status = %+v", resource.Status)
	}

	resource.Spec.Description = "recorded update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitClusterPlacementGroupConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Description != resource.Spec.Description ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated ClusterPlacementGroup status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	deleted = true

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func openRecordedClusterPlacementGroupSDK(
	t *testing.T,
	mode ocireplay.Mode,
) (clusterplacementgroupssdk.ClusterPlacementGroupsCPClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "clusterplacementgroups",
		Resource: "ClusterPlacementGroup",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "clusterplacementgroup_crud.yaml")

	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := clusterplacementgroupssdk.NewClusterPlacementGroupsCPClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path:       cassettePath,
			Metadata:   metadata,
			BaseClient: &client.BaseClient,
			Bindings: map[string]string{
				"availability-domain": requiredClusterPlacementGroupRecordingEnv(t, "OCI_AVAILABILITY_DOMAIN"),
			},
			Overwrite: ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}

	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path:     cassettePath,
		Host:     "https://clusterplacementgroups.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20230801",
		Metadata: metadata,
		Bindings: map[string]string{
			"availability-domain": "example:AD-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return clusterplacementgroupssdk.ClusterPlacementGroupsCPClient{
		BaseClient: session.BaseClient(),
	}, session.Close
}

func awaitClusterPlacementGroupConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	client ClusterPlacementGroupServiceClient,
	resource *clusterplacementgroupsv1beta1.ClusterPlacementGroup,
) error {
	return ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf(
				"ClusterPlacementGroup reconciliation was unsuccessful: %+v",
				response,
			)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredClusterPlacementGroupRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
