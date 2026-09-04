/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package esxihost

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	ocvpsdk "github.com/oracle/oci-go-sdk/v65/ocvp"
	ocvpv1beta1 "github.com/oracle/oci-service-operator/api/ocvp/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSyntheticEsxiHostCreateReadDelete(t *testing.T) {
	resource := newEsxiHostTestResource()
	createdBody, err := ocireplay.SyntheticObservedBody(resource.Spec, "ocid1.esxihost.oc1..synthetic", "ACTIVE", map[string]any{"compartmentId": "ocid1.compartment.oc1..replay"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := ocireplay.Metadata{Service: "ocvp", Resource: "EsxiHost", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKSynthetic(ocireplay.SDKSyntheticOptions{Path: filepath.Join("testdata", "recordings", "esxihost_synthetic_crud.yaml"), Host: "https://ocvp.us-ashburn-1.oci.oraclecloud.com", BasePath: "20230701", Metadata: metadata, Responder: ocireplay.NewSyntheticCRUDResponder(ocireplay.SyntheticCRUDOptions{CreatedBody: createdBody, PresentCollectionBody: `{"items":[` + createdBody + `]}`})})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := ocvpsdk.EsxiHostClient{BaseClient: session.BaseClient()}
	manager := &EsxiHostServiceManager{Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}}
	hooks := newEsxiHostRuntimeHooks(manager, sdkClient)
	client := wrapEsxiHostGeneratedClient(hooks, defaultEsxiHostServiceClient{ServiceClient: generatedruntime.NewServiceClient[*ocvpv1beta1.EsxiHost](buildEsxiHostGeneratedRuntimeConfig(manager, hooks))})
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*ocvpv1beta1.EsxiHost]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: generatedruntime.WithSkipExistingBeforeCreate, HasIdentity: func(current *ocvpv1beta1.EsxiHost) bool {
		return current.Status.OsokStatus.Ocid != "" || current.Status.Id != ""
	}, ValidateCreated: func(current *ocvpv1beta1.EsxiHost) error {
		if current.Status.Id == "" {
			return fmt.Errorf("created EsxiHost status = %+v", current.Status)
		}
		return nil
	}})
}
