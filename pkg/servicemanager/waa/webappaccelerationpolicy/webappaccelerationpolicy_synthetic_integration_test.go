/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package webappaccelerationpolicy

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	waasdk "github.com/oracle/oci-go-sdk/v65/waa"
	waav1beta1 "github.com/oracle/oci-service-operator/api/waa/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticWebAppAccelerationPolicyCreateReadDelete(t *testing.T) {
	resource := makeWebAppAccelerationPolicyResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.webappaccelerationpolicy.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "waa", Resource: "WebAppAccelerationPolicy", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "webappaccelerationpolicy_synthetic_crud.yaml"), Host: "https://waa.us-ashburn-1.oci.oraclecloud.com", BasePath: "20211230", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := syntheticWebAppAccelerationPolicyOCIClient{
		WaaClient:         waasdk.WaaClient{BaseClient: session.BaseClient()},
		WorkRequestClient: waasdk.WorkRequestClient{BaseClient: session.BaseClient()},
	}
	client := newWebAppAccelerationPolicyServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*waav1beta1.WebAppAccelerationPolicy]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *waav1beta1.WebAppAccelerationPolicy) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *waav1beta1.WebAppAccelerationPolicy) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created WebAppAccelerationPolicy status = %+v", current.Status)
		}
		return nil
	}})
}

type syntheticWebAppAccelerationPolicyOCIClient struct {
	waasdk.WaaClient
	waasdk.WorkRequestClient
}
