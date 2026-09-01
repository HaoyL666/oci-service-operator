/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package containerrepository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	artifactssdk "github.com/oracle/oci-go-sdk/v65/artifacts"
	artifactsv1beta1 "github.com/oracle/oci-service-operator/api/artifacts/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedContainerRepositoryCreateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	sdkClient, closeSession := openRecordedContainerRepositorySDK(t, mode)
	compartmentID := "ocid1.compartment.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = os.Getenv("OCI_COMPARTMENT_ID")
	}
	resource := &artifactsv1beta1.ContainerRepository{Spec: artifactsv1beta1.ContainerRepositorySpec{
		CompartmentId: compartmentID, DisplayName: "osok-replay-common-repository-v1",
		FreeformTags: map[string]string{"osok-replay": "create"},
	}}
	client := newContainerRepositoryServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: logr.Discard()}, sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		response, err := client.CreateOrUpdate(generatedruntime.WithSkipExistingBeforeCreate(ctx), resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("create failed: %+v", response)
		}
		return !response.ShouldRequeue, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := ocireplay.Await(ctx, mode, 5*time.Second, func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
}

func openRecordedContainerRepositorySDK(
	t *testing.T,
	mode ocireplay.Mode,
) (artifactssdk.ArtifactsClient, func() error) {
	t.Helper()
	metadata := ocireplay.Metadata{
		Service:  "artifacts",
		Resource: "ContainerRepository",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	path := filepath.Join("testdata", "recordings", "containerrepository_crud.yaml")
	if mode == ocireplay.ModeRecord {
		provider, err := ocireplay.RecordingConfigurationProvider()
		if err != nil {
			t.Fatal(err)
		}
		client, err := artifactssdk.NewArtifactsClientWithConfigurationProvider(provider)
		if err != nil {
			t.Fatal(err)
		}
		session, err := ocireplay.OpenSDKRecord(
			ocireplay.SDKRecordOptions{
				Path:       path,
				Metadata:   metadata,
				BaseClient: &client.BaseClient,
				Overwrite:  ocireplay.RecordingOverwriteRequested(),
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		return client, session.Close
	}
	session, err := ocireplay.OpenSDKReplay(
		ocireplay.SDKReplayOptions{
			Path:     path,
			Host:     "https://artifacts.us-ashburn-1.oci.oraclecloud.com",
			BasePath: "20160918",
			Metadata: metadata,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return artifactssdk.ArtifactsClient{BaseClient: session.BaseClient()}, session.Close
}
