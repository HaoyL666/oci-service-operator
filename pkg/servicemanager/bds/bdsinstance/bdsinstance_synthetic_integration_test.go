/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package bdsinstance

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	bdssdk "github.com/oracle/oci-go-sdk/v65/bds"
	bdsv1beta1 "github.com/oracle/oci-service-operator/api/bds/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// A BDS cluster allocates multiple paid compute nodes and block volumes, so
// the portable lifecycle uses a contract-faithful synthetic cassette.
func TestSyntheticBdsInstanceCreateUpdateDelete(t *testing.T) {
	metadata := ocireplay.Metadata{Service: "bds", Resource: "BdsInstance", Operations: []ocireplay.Operation{ocireplay.OperationCreate, ocireplay.OperationRead, ocireplay.OperationUpdate, ocireplay.OperationDelete}, SDKVersion: "v65.110.0", Provenance: ocireplay.ProvenanceSynthetic}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{Path: filepath.Join("testdata", "recordings", "bdsinstance_synthetic_crud.yaml"), Host: "https://bigdataservice.us-ashburn-1.oci.oraclecloud.com", BasePath: "20190531", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := bdssdk.BdsClient{BaseClient: session.BaseClient()}
	client := newBdsInstanceTestManager(sdkClient).client
	resource := makeSpecBdsInstance()
	ocireplay.RunLifecycle(t, ocireplay.LifecycleScenario[*bdsv1beta1.BdsInstance]{
		Mode: ocireplay.ModeReplay, Resource: resource, Client: client, CloseSession: session.Close, Timeout: time.Minute,
		CreateContext: func(ctx context.Context) context.Context { return generatedruntime.WithSkipExistingBeforeCreate(ctx) },
		HasIdentity:   func(current *bdsv1beta1.BdsInstance) bool { return current.Status.OsokStatus.Ocid != "" },
		ValidateCreated: func(current *bdsv1beta1.BdsInstance) error {
			if current.Status.DisplayName != "test-bds" || current.Status.LifecycleState != "ACTIVE" {
				return fmt.Errorf("created BdsInstance status = %+v", current.Status)
			}
			return nil
		},
		Mutate: func(current *bdsv1beta1.BdsInstance) { current.Spec.DisplayName = "test-bds-updated" },
		ValidateUpdated: func(current *bdsv1beta1.BdsInstance) error {
			if current.Status.DisplayName != "test-bds-updated" {
				return fmt.Errorf("updated BdsInstance status = %+v", current.Status)
			}
			return nil
		},
	})
}
