/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package routetable

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	coresdk "github.com/oracle/oci-go-sdk/v65/core"
	corev1beta1 "github.com/oracle/oci-service-operator/api/core/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedRouteTableName = "osok-replay-common-route-table-v1"

func TestRecordedRouteTableCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedRouteTableSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close RouteTable cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	vcnID := "ocid1.vcn.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredRouteTableRecordingEnv(t, "OCI_COMPARTMENT_ID")
		vcnID = requiredRouteTableRecordingEnv(t, "OCI_REPLAY_VCN_ID")
	}
	resource := &corev1beta1.RouteTable{Spec: corev1beta1.RouteTableSpec{
		CompartmentId: compartmentID,
		VcnId:         vcnID,
		DisplayName:   recordedRouteTableName,
		RouteRules:    []corev1beta1.RouteTableRouteRule{},
		FreeformTags:  map[string]string{"osok-replay": "create"},
	}}
	manager := newRouteTableTestManager(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
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
				return manager.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitRouteTableConvergence(createCtx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(coresdk.RouteTableLifecycleStateAvailable) ||
		resource.Status.DisplayName != recordedRouteTableName {
		t.Fatalf("created RouteTable status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedRouteTableName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitRouteTableConvergence(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated RouteTable status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		return manager.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	deleted = true
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func openRecordedRouteTableSDK(t *testing.T, mode ocireplay.Mode) (coresdk.VirtualNetworkClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service: "core", Resource: "RouteTable",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded,
	}
	cassettePath := filepath.Join("testdata", "recordings", "routetable_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := coresdk.NewVirtualNetworkClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{
			Path: cassettePath, Metadata: metadata, BaseClient: &client.BaseClient,
			Overwrite: ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: cassettePath, Host: "https://iaas.us-ashburn-1.oraclecloud.com",
		BasePath: "20160918", Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return coresdk.VirtualNetworkClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitRouteTableConvergence(
	ctx context.Context,
	mode ocireplay.Mode,
	manager *RouteTableServiceManager,
	resource *corev1beta1.RouteTable,
) error {
	return ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("RouteTable reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredRouteTableRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
