/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package oceinstance

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	ocesdk "github.com/oracle/oci-go-sdk/v65/oce"
	ocev1beta1 "github.com/oracle/oci-service-operator/api/oce/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// OCE instances allocate paid managed content infrastructure and require IDCS provisioning.
func TestSyntheticOceInstanceCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "oce", Resource: "OceInstance", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: filepath.Join("testdata", "recordings", "oceinstance_synthetic_crud.yaml"), Host: "https://cp.oce.us-ashburn-1.ocp.oraclecloud.com", BasePath: "20190912", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := ocesdk.OceInstanceClient{BaseClient: session.BaseClient()}
	client := newOceInstanceServiceClientWithOCIClient(loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")}, sdkClient)
	resource := newOceInstanceTestResource()
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*ocev1beta1.OceInstance]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) }, HasIdentity: func(current *ocev1beta1.OceInstance) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *ocev1beta1.OceInstance) error {
		if current.Status.Name != "oce-runtime-test" || current.Status.LifecycleState != "ACTIVE" {
			return fmt.Errorf("created OceInstance status = %+v", current.Status)
		}
		return nil
	}, Mutate: func(current *ocev1beta1.OceInstance) { current.Spec.Description = "runtime test instance updated" }, ValidateUpdated: func(current *ocev1beta1.OceInstance) error {
		if current.Status.Description != "runtime test instance updated" {
			return fmt.Errorf("updated OceInstance status = %+v", current.Status)
		}
		return nil
	}})
}
