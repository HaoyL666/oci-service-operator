/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package lockbox

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	lockboxsdk "github.com/oracle/oci-go-sdk/v65/lockbox"
	lockboxv1beta1 "github.com/oracle/oci-service-operator/api/lockbox/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticLockboxCreateReadDelete(t *testing.T) {
	resource := baseLockboxResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.lockbox.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "lockbox", Resource: "Lockbox", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "lockbox_synthetic_crud.yaml"), Host: "https://lockbox.us-ashburn-1.oci.oraclecloud.com", BasePath: "20220126", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := lockboxsdk.LockboxClient{BaseClient: session.BaseClient()}
	client := newLockboxServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*lockboxv1beta1.Lockbox]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *lockboxv1beta1.Lockbox) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *lockboxv1beta1.Lockbox) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created Lockbox status = %+v", current.Status)
		}
		return nil
	}})
}
