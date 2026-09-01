/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package bastion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	bastionsdk "github.com/oracle/oci-go-sdk/v65/bastion"
	bastionv1beta1 "github.com/oracle/oci-service-operator/api/bastion/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const recordedBastionName = "osok-replay-common-bastion-v1"

func TestRecordedBastionCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedBastionSDK(t, mode)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Bastion cassette: %v", err)
			}
		}
	})

	compartmentID := "ocid1.compartment.oc1..replay"
	subnetID := "ocid1.subnet.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredBastionRecordingEnv(t, "OCI_COMPARTMENT_ID")
		subnetID = requiredBastionRecordingEnv(t, "OCI_BASTION_SUBNET_ID")
	}
	resource := &bastionv1beta1.Bastion{Spec: bastionv1beta1.BastionSpec{
		BastionType:              "STANDARD",
		CompartmentId:            compartmentID,
		TargetSubnetId:           subnetID,
		Name:                     recordedBastionName,
		ClientCidrBlockAllowList: []string{"0.0.0.0/0"},
		MaxSessionTtlInSeconds:   1800,
		DnsProxyStatus:           "DISABLED",
		FreeformTags:             map[string]string{"osok-replay": "create"},
	}}
	client := newBastionServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("recorded-integration")},
		sdkClient,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.Id == "" {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cleanupCancel()
			_ = ocireplay.Await(cleanupCtx, mode, 10*time.Second, func() (bool, error) {
				return client.Delete(cleanupCtx, resource)
			})
		})
	}

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitBastionConvergence(createCtx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(bastionsdk.BastionLifecycleStateActive) ||
		resource.Status.Name != recordedBastionName {
		t.Fatalf("created Bastion status = %+v", resource.Status)
	}

	resource.Spec.ClientCidrBlockAllowList = []string{"10.0.0.0/8"}
	resource.Spec.MaxSessionTtlInSeconds = 3600
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitBastionConvergence(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.MaxSessionTtlInSeconds != 3600 ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Bastion status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, mode, bastionPollInterval(mode), func() (bool, error) {
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

func openRecordedBastionSDK(t *testing.T, mode ocireplay.Mode) (bastionsdk.BastionClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "bastion", Resource: "Bastion", Operations: []ocireplay.Operation{
		ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete,
	}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	cassettePath := filepath.Join("testdata", "recordings", "bastion_crud.yaml")
	bindings := map[string]string{"subnet": "ocid1.subnet.oc1..replay"}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := bastionsdk.NewBastionClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		bindings["subnet"] = requiredBastionRecordingEnv(t, "OCI_BASTION_SUBNET_ID")
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: cassettePath, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: cassettePath, Host: "https://bastion.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210331", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return bastionsdk.BastionClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitBastionConvergence(ctx context.Context, mode ocireplay.Mode, client BastionServiceClient, resource *bastionv1beta1.Bastion) error {
	return ocireplay.Await(ctx, mode, bastionPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Bastion reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func bastionPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 10 * time.Second
	}
	return 0
}

func requiredBastionRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
