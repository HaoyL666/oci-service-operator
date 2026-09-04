/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package jmsplugin

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	jmssdk "github.com/oracle/oci-go-sdk/v65/jms"
	jmsv1beta1 "github.com/oracle/oci-service-operator/api/jms/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticJmsPluginCreateReadDelete(t *testing.T) {
	resource := makeJmsPluginResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.jmsplugin.oc1..synthetic", "ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "jms", Resource: "JmsPlugin", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "jmsplugin_synthetic_crud.yaml"), Host: "https://javamanagementservice.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210610", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := jmssdk.JavaManagementServiceClient{BaseClient: session.BaseClient()}
	client := newJmsPluginServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*jmsv1beta1.JmsPlugin]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *jmsv1beta1.JmsPlugin) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *jmsv1beta1.JmsPlugin) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created JmsPlugin status = %+v", current.Status)
		}
		return nil
	}})
}
