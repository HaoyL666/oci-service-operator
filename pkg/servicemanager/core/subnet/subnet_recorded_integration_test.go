/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package subnet

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

const recordedSubnetName = "osok-replay-basic-subnet-v1"

func TestRecordedSubnetCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedSubnetSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Subnet cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	vcnID := "ocid1.vcn.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredSubnetRecordingEnv(t, "OCI_COMPARTMENT_ID")
		vcnID = requiredSubnetRecordingEnv(t, "OCI_REPLAY_VCN_ID")
	}
	resource := &corev1beta1.Subnet{
		Spec: corev1beta1.SubnetSpec{
			CidrBlock:               "10.96.1.0/24",
			CompartmentId:           compartmentID,
			VcnId:                   vcnID,
			DisplayName:             recordedSubnetName,
			DnsLabel:                "replaysub",
			ProhibitPublicIpOnVnic:  true,
			ProhibitInternetIngress: true,
			FreeformTags:            map[string]string{"osok-replay": "create"},
		},
	}
	manager := newTestManager(sdkClient)
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
	if err := awaitSubnetConvergence(createCtx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(coresdk.SubnetLifecycleStateAvailable) || resource.Status.DisplayName != recordedSubnetName || !resource.Status.ProhibitPublicIpOnVnic {
		t.Fatalf("created Subnet status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = recordedSubnetName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitSubnetConvergence(ctx, mode, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName || resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Subnet status = %+v", resource.Status)
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

func openRecordedSubnetSDK(t *testing.T, mode ocireplay.Mode) (coresdk.VirtualNetworkClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service:    "core",
		Resource:   "Subnet",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "subnet_crud.yaml")
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
			Path:       path,
			Metadata:   metadata,
			BaseClient: &client.BaseClient,
			Overwrite:  ocireplay.RecordingOverwriteRequested(),
		})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path:     path,
		Host:     "https://iaas.us-ashburn-1.oraclecloud.com",
		BasePath: "20160918",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return coresdk.VirtualNetworkClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitSubnetConvergence(ctx context.Context, mode ocireplay.Mode, manager *SubnetServiceManager, resource *corev1beta1.Subnet) error {
	return ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Subnet reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func requiredSubnetRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
