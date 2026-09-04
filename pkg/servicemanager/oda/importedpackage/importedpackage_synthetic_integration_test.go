/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package importedpackage

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	odasdk "github.com/oracle/oci-go-sdk/v65/oda"
	odav1beta1 "github.com/oracle/oci-service-operator/api/oda/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestSyntheticImportedPackageCreateReadDelete(t *testing.T) {
	resource := makeImportedPackageResource("ocid1.odainstance.oc1..synthetic", "ocid1.odapackage.oc1..synthetic")
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.importedpackage.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "oda", Resource: "ImportedPackage", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "importedpackage_synthetic_crud.yaml"), Host: "https://digitalassistant-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190506", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CollectionPath: "/20190506/odaInstances/ocid1.odainstance.oc1..synthetic/importedPackages", CreatedBody: createdBody, EmptyCollectionBody: "[]", PresentCollectionBody: "[" + createdBody + "]"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := odasdk.OdapackageClient{BaseClient: session.BaseClient()}
	client := newTestImportedPackageClient(sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*odav1beta1.ImportedPackage]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *odav1beta1.ImportedPackage) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *odav1beta1.ImportedPackage) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created ImportedPackage status = %+v", current.Status)
		}
		return nil
	}})
}
