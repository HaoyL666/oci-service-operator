/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package vbinstance

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	visualbuildersdk "github.com/oracle/oci-go-sdk/v65/visualbuilder"
	visualbuilderv1beta1 "github.com/oracle/oci-service-operator/api/visualbuilder/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// A Visual Builder instance allocates paid managed application capacity.
func TestSyntheticVbInstanceCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "visualbuilder", Resource: "VbInstance", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: filepath.Join("testdata", "recordings", "vbinstance_synthetic_crud.yaml"), Host: "https://visualbuilder.us-ashburn-1.ocp.oraclecloud.com", BasePath: "20210601", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := visualbuildersdk.VbInstanceClient{BaseClient: session.BaseClient()}
	client := newVbInstanceServiceClientWithOCIClient(sdkClient)
	resource := newMinimalTestVbInstance()
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*visualbuilderv1beta1.VbInstance]{Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute, CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) }, HasIdentity: func(current *visualbuilderv1beta1.VbInstance) bool { return current.Status.OsokStatus.Ocid != "" }, ValidateCreated: func(current *visualbuilderv1beta1.VbInstance) error {
		if current.Status.DisplayName != "vb-instance-minimal" || current.Status.LifecycleState != "ACTIVE" {
			return fmt.Errorf("created VbInstance status = %+v", current.Status)
		}
		return nil
	}, Mutate: func(current *visualbuilderv1beta1.VbInstance) { current.Spec.DisplayName = "vb-instance-updated" }, ValidateUpdated: func(current *visualbuilderv1beta1.VbInstance) error {
		if current.Status.DisplayName != "vb-instance-updated" {
			return fmt.Errorf("updated VbInstance status = %+v", current.Status)
		}
		return nil
	}})
}
