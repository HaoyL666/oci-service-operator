/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package discoveryjob

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	stackmonitoringsdk "github.com/oracle/oci-go-sdk/v65/stackmonitoring"
	stackmonitoringv1beta1 "github.com/oracle/oci-service-operator/api/stackmonitoring/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticDiscoveryJobCreateReadDelete(t *testing.T) {
	resource := makeDiscoveryJobResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.stackmonitoringdiscoveryjob.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "stackmonitoring", Resource: "DiscoveryJob", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "discoveryjob_synthetic_crud.yaml"), Host: "https://stack-monitoring.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210330", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := stackmonitoringsdk.StackMonitoringClient{BaseClient: session.BaseClient()}
	client := newDiscoveryJobServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*stackmonitoringv1beta1.DiscoveryJob]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *stackmonitoringv1beta1.DiscoveryJob) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *stackmonitoringv1beta1.DiscoveryJob) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created DiscoveryJob status = %+v", current.Status)
		}
		return nil
	}})
}
