/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package loganalyticsembridge

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	loganalyticssdk "github.com/oracle/oci-go-sdk/v65/loganalytics"
	loganalyticsv1beta1 "github.com/oracle/oci-service-operator/api/loganalytics/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticLogAnalyticsEmBridgeCreateReadDelete(t *testing.T) {
	resource := newLogAnalyticsEmBridgeResource()
	resource.Annotations = map[string]string{logAnalyticsEmBridgeNamespaceAnnotation: "osokreplaynamespace"}
	createdBody, err := ocireplay.SyntheticJSONBody(newSDKLogAnalyticsEmBridge(testLogAnalyticsEmBridgeID, resource.Spec.BucketName, loganalyticssdk.EmBridgeLifecycleStatesActive))
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "loganalytics", Resource: "LogAnalyticsEmBridge", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "loganalyticsembridge_synthetic_crud.yaml"), Host: "https://loganalytics.us-ashburn-1.oci.oraclecloud.com", BasePath: "20200601", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: "{\"items\":[" + createdBody + "]}"})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := loganalyticssdk.LogAnalyticsClient{BaseClient: session.BaseClient()}
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}
	provider := common.NewRawConfigurationProvider("ocid1.tenancy.oc1..synthetic", "ocid1.user.oc1..synthetic", "us-ashburn-1", "00:00:00", "unused", nil)
	client := newLogAnalyticsEmBridgeServiceClientWithOCIClient(log, provider, sdkClient)
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*loganalyticsv1beta1.LogAnalyticsEmBridge]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *loganalyticsv1beta1.LogAnalyticsEmBridge) bool {
		return current.Status.OsokStatus.Ocid != ""
	}, ValidateCreated: func(current *loganalyticsv1beta1.LogAnalyticsEmBridge) error {
		if current.Status.OsokStatus.Ocid == "" {
			return fmt.Errorf("created LogAnalyticsEmBridge status = %+v", current.Status)
		}
		return nil
	}})
}
