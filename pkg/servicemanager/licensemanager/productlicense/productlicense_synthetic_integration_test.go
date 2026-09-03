/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package productlicense

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	licensemanagersdk "github.com/oracle/oci-go-sdk/v65/licensemanager"
	licensemanagerv1beta1 "github.com/oracle/oci-service-operator/api/licensemanager/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func TestSyntheticProductLicenseCreateReadDelete(t *testing.T) {
	resource := productLicenseResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.productlicense.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{
		Service: "licensemanager", Resource: "ProductLicense",
		Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete},
		SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{
		Path: filepath.Join("testdata", "recordings", "productlicense_synthetic_crud.yaml"), Host: "https://licensemanager.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220430", Metadata: metadata,
		Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := licensemanagersdk.LicenseManagerClient{BaseClient: session.BaseClient()}
	client := newProductLicenseServiceClientWithOCIClient(sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*licensemanagerv1beta1.ProductLicense]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		HasIdentity:   func(current *licensemanagerv1beta1.ProductLicense) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *licensemanagerv1beta1.ProductLicense) error {
			if current.Status.OsokStatus.Ocid == "" {
				return fmt.Errorf("created ProductLicense status = %+v", current.Status)
			}
			return nil
		},
	})
}
