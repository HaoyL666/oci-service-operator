/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package vault

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	keymanagementsdk "github.com/oracle/oci-go-sdk/v65/keymanagement"
	keymanagementv1beta1 "github.com/oracle/oci-service-operator/api/keymanagement/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticVaultCreateRead(t *testing.T) {
	resource := &keymanagementv1beta1.Vault{Spec: keymanagementv1beta1.VaultSpec{
		CompartmentId: "ocid1.compartment.oc1..replay", DisplayName: "osok-replay-vault", VaultType: "DEFAULT",
	}}
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.vault.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "keymanagement", Resource: "Vault", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "vault_synthetic_crud.yaml"), Host: "https://kms.us-ashburn-1.oraclecloud.com", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := keymanagementsdk.KmsVaultClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	delegate := defaultVaultServiceClient{ServiceClient: generatedruntime.NewServiceClient[*keymanagementv1beta1.Vault](newVaultRuntimeConfig(log, sdkClient))}
	client := &vaultRuntimeClient{delegate: delegate, sdk: sdkClient, log: log}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	response, err := client.CreateOrUpdate(generatedruntime.WithSkipExistingBeforeCreate(ctx), resource, ctrl.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !response.IsSuccessful || response.ShouldRequeue || resource.Status.OsokStatus.Ocid == "" || resource.Status.DisplayName != resource.Spec.DisplayName {
		t.Fatal(fmt.Errorf("created Vault response = %+v status = %+v", response, resource.Status))
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
