/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package auditarchiveretrieval

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	datasafesdk "github.com/oracle/oci-go-sdk/v65/datasafe"
	datasafev1beta1 "github.com/oracle/oci-service-operator/api/datasafe/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestSyntheticAuditArchiveRetrievalCreateReadDelete(t *testing.T) {
	resource := newAuditArchiveRetrievalTestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.auditarchiveretrieval.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "datasafe", Resource: "AuditArchiveRetrieval", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "auditarchiveretrieval_synthetic_crud.yaml"), Host: "https://datasafe.us-ashburn-1.oci.oraclecloud.com", BasePath: "20181201", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := datasafesdk.DataSafeClient{BaseClient: session.BaseClient()}
	hooks := newAuditArchiveRetrievalDefaultRuntimeHooks(sdkClient)
	applyAuditArchiveRetrievalRuntimeHooks(&hooks)
	client := newAuditArchiveRetrievalRuntimeTestClient(hooks)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*datasafev1beta1.AuditArchiveRetrieval]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *datasafev1beta1.AuditArchiveRetrieval) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *datasafev1beta1.AuditArchiveRetrieval) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created AuditArchiveRetrieval status = %+v", current.Status)
		}
		return nil
	}})
}
