/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package opainstance

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	opasdk "github.com/oracle/oci-go-sdk/v65/opa"
	opav1beta1 "github.com/oracle/oci-service-operator/api/opa/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// OPA instances allocate paid process-automation capacity and require IDCS provisioning.
func TestSyntheticOpaInstanceCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "opa", Resource: "OpaInstance", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: filepath.Join("testdata", "recordings", "opainstance_synthetic_crud.yaml"), Host: "https://process.us-ashburn-1.oci.oraclecloud.com", BasePath: "20210621", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := opasdk.OpaInstanceClient{BaseClient: session.BaseClient()}
	client := newOpaInstanceServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	resource := makeOpaInstanceResource()
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*opav1beta1.OpaInstance]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) }, HasIdentity: func(current *opav1beta1.OpaInstance) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *opav1beta1.OpaInstance) error {
		if current.Status.DisplayName != "opa-instance" || current.Status.LifecycleState != "ACTIVE" {
			return fmt.Errorf("created OpaInstance status = %+v", current.Status)
		}
		return nil
	}, Mutate: func(current *opav1beta1.OpaInstance) { current.Spec.Description = "opa instance updated" }, ValidateUpdated: func(current *opav1beta1.OpaInstance) error {
		if current.Status.Description != "opa instance updated" {
			return fmt.Errorf("updated OpaInstance status = %+v", current.Status)
		}
		return nil
	}})
}
