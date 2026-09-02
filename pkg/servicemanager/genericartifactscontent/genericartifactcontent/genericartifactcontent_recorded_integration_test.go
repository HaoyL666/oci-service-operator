/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package genericartifactcontent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	genericartifactscontentv1beta1 "github.com/oracle/oci-service-operator/api/genericartifactscontent/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRecordedGenericArtifactContentReadAndRelease(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:    "genericartifactscontent",
		Resource:   "GenericArtifactContent",
		Operations: []ocireplay.Operation{ocireplay.OperationRead},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	artifactID := "ocid1.genericartifact.oc1..replay"
	if mode == ocireplay.ModeRecord {
		artifactID = requiredGenericArtifactContentEnv(t, "OCI_REPLAY_GENERIC_ARTIFACT_ID")
	}
	sdkClient, closeSession := ocireplay.OpenGenericArtifactsContentSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "genericartifactcontent_read.yaml"),
		metadata,
	)
	resource := &genericartifactscontentv1beta1.GenericArtifactContent{}
	resource.Status.OsokStatus.Ocid = shared.OCID(artifactID)
	client := newGenericArtifactContentRuntimeClientWithOCIClient(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("CreateOrUpdate() response = %+v", response)
	}
	if resource.Status.OsokStatus.Ocid != shared.OCID(artifactID) || resource.Status.OsokStatus.Reason != string(shared.Active) {
		t.Fatalf("read GenericArtifactContent status = %+v", resource.Status)
	}

	deleted, err := client.Delete(ctx, resource)
	if err != nil {
		t.Fatal(err)
	}
	if !deleted || resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatalf("release GenericArtifactContent deleted=%t status=%+v", deleted, resource.Status)
	}
	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
}

func requiredGenericArtifactContentEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
