/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package quota

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	limitssdk "github.com/oracle/oci-go-sdk/v65/limits"
	limitsv1beta1 "github.com/oracle/oci-service-operator/api/limits/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestSyntheticQuotaCreateReadDelete(t *testing.T) {
	resource := newQuotaRuntimeTestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.quota.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "limits", Resource: "Quota", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "quota_synthetic_crud.yaml"), Host: "https://limits.us-ashburn-1.oci.oraclecloud.com", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := limitssdk.QuotasClient{BaseClient: session.BaseClient()}
	client := newQuotaRuntimeTestClient(&fakeQuotaOCIClient{createFunc: sdkClient.CreateQuota, getFunc: sdkClient.GetQuota, listFunc: sdkClient.ListQuotas, updateFunc: sdkClient.UpdateQuota, deleteFunc: sdkClient.DeleteQuota})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*limitsv1beta1.Quota]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *limitsv1beta1.Quota) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *limitsv1beta1.Quota) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created Quota status = %+v", current.Status)
		}
		return nil
	}})
}
