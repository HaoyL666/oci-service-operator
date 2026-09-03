/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package licenserecord

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	licensemanagersdk "github.com/oracle/oci-go-sdk/v65/licensemanager"
	licensemanagerv1beta1 "github.com/oracle/oci-service-operator/api/licensemanager/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticLicenseRecordCreateReadDelete(t *testing.T) {
	resource := makeLicenseRecordResource()
	created, err := json.Marshal(makeSDKLicenseRecord("ocid1.licenserecord.oc1..synthetic", resource.Spec, licensemanagersdk.LifeCycleStateActive))
	if err != nil {
		t.Fatal(err)
	}
	createdBody := string(created)
	metadata := ocireplay.Metadata{Service: "licensemanager", Resource: "LicenseRecord", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "licenserecord_synthetic_crud.yaml"), Host: "https://licensemanager.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220430", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := licensemanagersdk.LicenseManagerClient{BaseClient: session.BaseClient()}
	client := newLicenseRecordServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*licensemanagerv1beta1.LicenseRecord]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *licensemanagerv1beta1.LicenseRecord) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *licensemanagerv1beta1.LicenseRecord) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created LicenseRecord status = %+v", current.Status)
		}
		return nil
	}})
}
