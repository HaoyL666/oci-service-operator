/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package odainstance

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	odasdk "github.com/oracle/oci-go-sdk/v65/oda"
	odav1beta1 "github.com/oracle/oci-service-operator/api/oda/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// An ODA instance allocates paid Digital Assistant service capacity.
func TestSyntheticOdaInstanceCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "oda", Resource: "OdaInstance", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: filepath.Join("testdata", "recordings", "odainstance_synthetic_crud.yaml"), Host: "https://digitalassistant-api.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190506", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := odasdk.OdaClient{BaseClient: session.BaseClient()}
	manager := &OdaInstanceServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newOdaInstanceRuntimeHooks(manager, sdkClient)
	client := wrapOdaInstanceGeneratedClient(hooks, defaultOdaInstanceServiceClient{ServiceClient: generatedruntime.NewServiceClient[*odav1beta1.OdaInstance](buildOdaInstanceGeneratedRuntimeConfig(manager, hooks))})
	resource := newOdaInstanceTestResource()
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*odav1beta1.OdaInstance]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) }, HasIdentity: func(current *odav1beta1.OdaInstance) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *odav1beta1.OdaInstance) error {
		if current.Status.DisplayName != "oda-sample" || current.Status.LifecycleState != "ACTIVE" {
			return fmt.Errorf("created OdaInstance status = %+v", current.Status)
		}
		return nil
	}, Mutate: func(current *odav1beta1.OdaInstance) { current.Spec.Description = "ODA description updated" }, ValidateUpdated: func(current *odav1beta1.OdaInstance) error {
		if current.Status.Description != "ODA description updated" {
			return fmt.Errorf("updated OdaInstance status = %+v", current.Status)
		}
		return nil
	}})
}
