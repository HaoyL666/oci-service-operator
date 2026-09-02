/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package genericartifactcontentbypath

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	genericartifactscontentv1beta1 "github.com/oracle/oci-service-operator/api/genericartifactscontent/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRecordedGenericArtifactContentByPathCreateUpdateDelete(t *testing.T) {
	mode, err := ocireplay.RequestedMode()
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service:  "genericartifactscontent",
		Resource: "GenericArtifactContentByPath",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceRecorded,
	}
	compartmentID := "ocid1.compartment.oc1..replay"
	repositoryID := "ocid1.artifactrepository.oc1..replay"
	if mode == ocireplay.ModeRecord {
		compartmentID = requiredGenericArtifactContentByPathEnv(t, "OCI_COMPARTMENT_ID")
		repositoryID = requiredGenericArtifactContentByPathEnv(t, "OCI_REPLAY_GENERIC_REPOSITORY_ID")
	}
	clients, closeSession := ocireplay.OpenGenericArtifactContentByPathSDK(
		t,
		mode,
		filepath.Join("testdata", "recordings", "genericartifactcontentbypath_crud.yaml"),
		metadata,
		map[string]string{"compartment": compartmentID},
	)
	credentials := fakeGenericArtifactContentByPathCredentialClient{
		secrets: map[string]map[string][]byte{
			"default/osok-replay-generic-content": {
				genericArtifactContentByPathDefaultContentKey: []byte("OSOK recorded artifact content v1"),
			},
		},
	}
	client := &genericArtifactContentByPathRuntimeClient{
		contentClient:    clients.Content,
		artifactClient:   clients.Artifacts,
		credentialClient: credentials,
	}
	resource := &genericartifactscontentv1beta1.GenericArtifactContentByPath{
		ObjectMeta: metav1.ObjectMeta{Name: "osok-replay-generic-content", Namespace: "default"},
		Spec: genericartifactscontentv1beta1.GenericArtifactContentByPathSpec{
			CompartmentId: compartmentID,
			RepositoryId:  repositoryID,
			ArtifactPath:  "osok/replay-content-v2.txt",
			Version:       "1.0.0",
			Content: shared.SecretSource{
				SecretName: "osok-replay-generic-content",
			},
			ContentKey: genericArtifactContentByPathDefaultContentKey,
		},
	}
	createdDigest := ""
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*genericartifactscontentv1beta1.GenericArtifactContentByPath]{
		Mode:         mode,
		Resource:     resource,
		Client:       client,
		CloseSession: closeSession,
		PollInterval: 5 * time.Second,
		Timeout:      15 * time.Minute,
		HasIdentity: func(current *genericartifactscontentv1beta1.GenericArtifactContentByPath) bool {
			return current.Status.OsokStatus.Ocid != ""
		},
		ValidateCreated: func(current *genericartifactscontentv1beta1.GenericArtifactContentByPath) error {
			if current.Status.Id == "" || current.Status.Sha256 == "" || current.Status.LifecycleState != "AVAILABLE" {
				return fmt.Errorf("created GenericArtifactContentByPath status = %+v", current.Status)
			}
			createdDigest = current.Status.Sha256
			return nil
		},
		Mutate: func(_ *genericartifactscontentv1beta1.GenericArtifactContentByPath) {
			credentials.secrets["default/osok-replay-generic-content"][genericArtifactContentByPathDefaultContentKey] = []byte("OSOK recorded artifact content v2")
		},
		ValidateUpdated: func(current *genericartifactscontentv1beta1.GenericArtifactContentByPath) error {
			if current.Status.Sha256 == "" || current.Status.Sha256 == createdDigest {
				return fmt.Errorf("updated GenericArtifactContentByPath status = %+v", current.Status)
			}
			return nil
		},
	})
}

func requiredGenericArtifactContentByPathEnv(t *testing.T, name string) string {
	t.Helper()
	if value := os.Getenv(name); value != "" {
		return value
	}
	t.Fatalf("%s is required in record mode", name)
	return ""
}
