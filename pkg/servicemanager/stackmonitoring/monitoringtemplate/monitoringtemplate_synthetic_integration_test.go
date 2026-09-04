/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package monitoringtemplate

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

func TestSyntheticMonitoringTemplateCreateReadDelete(t *testing.T) {
	resource := testMonitoringTemplate()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.monitoringtemplate.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "stackmonitoring", Resource: "MonitoringTemplate", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "monitoringtemplate_synthetic_crud.yaml"), Host: "https://stack-monitoring.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210330", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := stackmonitoringsdk.StackMonitoringClient{BaseClient: session.BaseClient()}
	manager := &MonitoringTemplateServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newMonitoringTemplateRuntimeHooks(manager, sdkClient)
	client := wrapMonitoringTemplateGeneratedClient(hooks, defaultMonitoringTemplateServiceClient{ServiceClient: generatedruntime.NewServiceClient[*stackmonitoringv1beta1.MonitoringTemplate](buildMonitoringTemplateGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*stackmonitoringv1beta1.MonitoringTemplate]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *stackmonitoringv1beta1.MonitoringTemplate) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *stackmonitoringv1beta1.MonitoringTemplate) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created MonitoringTemplate status = %+v", current.Status)
		}
		return nil
	}})
}
