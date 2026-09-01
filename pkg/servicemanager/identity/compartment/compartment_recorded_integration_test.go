/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package compartment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/oracle/oci-go-sdk/v65/common"
	identitysdk "github.com/oracle/oci-go-sdk/v65/identity"
	identityv1beta1 "github.com/oracle/oci-service-operator/api/identity/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedCompartmentCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	name := "osok-replay-compartment-replay"
	if mode == ocireplay.ModeRecord {
		name = fmt.Sprintf("osok-replay-compartment-%d", time.Now().UnixNano())
	}
	sdkClient, closeSession := openRecordedCompartmentSDK(t, mode, name)
	closed := false
	t.Cleanup(func() {
		if !closed && mode == ocireplay.ModeReplay {
			if err := closeSession(); err != nil {
				t.Errorf("close Compartment cassette: %v", err)
			}
		}
	})
	parentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		parentID = requiredCompartmentRecordingEnv(t, "OCI_COMPARTMENT_ID")
	}
	resource := &identityv1beta1.Compartment{Spec: identityv1beta1.CompartmentSpec{CompartmentId: parentID, Name: name, Description: "recorded create", FreeformTags: map[string]string{"osok-replay": "create"}}}
	client := newRecordedCompartmentClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	deleted := false
	if mode == ocireplay.ModeRecord {
		t.Cleanup(func() {
			if deleted || resource.Status.Id == "" {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cleanupCancel()
			_, _ = client.Delete(cleanupCtx, resource)
		})
	}
	if err := awaitRecordedCompartment(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Name != name || resource.Status.Id == "" {
		t.Fatalf("created Compartment status = %+v", resource.Status)
	}
	resource.Spec.Description = "recorded update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitRecordedCompartment(ctx, mode, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Description != resource.Spec.Description {
		t.Fatalf("updated Compartment status = %+v", resource.Status)
	}
	deleted, err = client.Delete(ctx, resource)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("Compartment delete was not accepted")
	}
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func newRecordedCompartmentClient(sdkClient identitysdk.IdentityClient) CompartmentServiceClient {
	manager := &CompartmentServiceManager{Log: loggerutil.OSOKLogger{Logger: logr.Discard()}}
	hooks := newCompartmentRuntimeHooks(manager, sdkClient)
	delegate := wrapCompartmentGeneratedClient(hooks, defaultCompartmentServiceClient{ServiceClient: generatedruntime.NewServiceClient[*identityv1beta1.Compartment](buildCompartmentGeneratedRuntimeConfig(manager, hooks))})
	return compartmentOrphanDeleteClient{
		delegate: delegate,
		deleteCompartment: func(ctx context.Context, compartmentID shared.OCID) error {
			_, err := sdkClient.DeleteCompartment(ctx, identitysdk.DeleteCompartmentRequest{CompartmentId: common.String(string(compartmentID))})
			return err
		},
		loadCompartment: func(ctx context.Context, compartmentID shared.OCID) (*identitysdk.Compartment, error) {
			response, err := sdkClient.GetCompartment(ctx, identitysdk.GetCompartmentRequest{CompartmentId: common.String(string(compartmentID))})
			if err != nil {
				return nil, err
			}
			return &response.Compartment, nil
		},
		listCompartments: func(ctx context.Context, parentID shared.OCID, name string) ([]identitysdk.Compartment, error) {
			response, err := sdkClient.ListCompartments(ctx, identitysdk.ListCompartmentsRequest{CompartmentId: common.String(string(parentID)), Name: common.String(name)})
			if err != nil {
				return nil, err
			}
			return response.Items, nil
		},
	}
}

func openRecordedCompartmentSDK(t *testing.T, mode ocireplay.Mode, name string) (identitysdk.IdentityClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{Service: "identity", Resource: "Compartment", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceRecorded}
	path := filepath.Join("testdata", "recordings", "compartment_crud.yaml")
	bindings := map[string]string{"compartment-name": name}
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := identitysdk.NewIdentityClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(ocireplay.SDKRecordOptions{Path: path, Metadata: metadata, BaseClient: &client.BaseClient, Bindings: bindings, Overwrite: ocireplay.RecordingOverwriteRequested()})
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: path, Host: "https://identity.us-ashburn-1.oci.oraclecloud.com", BasePath: "20160918", Metadata: metadata, Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	return identitysdk.IdentityClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitRecordedCompartment(ctx context.Context, mode ocireplay.Mode, client CompartmentServiceClient, resource *identityv1beta1.Compartment) error {
	return ocireplay.Await(ctx, mode, recordedCompartmentPollInterval(mode), func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			if recordedCompartmentIsTransientNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Compartment reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}

func recordedCompartmentIsTransientNotFound(err error) bool {
	return compartmentDeleteIsNotFound(err)
}
func recordedCompartmentPollInterval(mode ocireplay.Mode) time.Duration {
	if mode == ocireplay.ModeRecord {
		return 5 * time.Second
	}
	return 0
}
func requiredCompartmentRecordingEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required in record mode", name)
	}
	return value
}
