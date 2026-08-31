/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package governanceinstance

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	accessgovernancecpsdk "github.com/oracle/oci-go-sdk/v65/accessgovernancecp"
	accessgovernancecpv1beta1 "github.com/oracle/oci-service-operator/api/accessgovernancecp/v1beta1"
	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const syntheticGovernanceInstanceName = "osok-replay-synthetic-governance-v1"

// Access Governance requires a service entitlement and an IDCS administrator
// token. The synthetic cassette exercises the published SDK contract without
// claiming that this lifecycle was observed in a live tenancy.
func TestSyntheticGovernanceInstanceCreateUpdateDelete(t *testing.T) {
	sdkClient, closeSession := openSyntheticGovernanceInstanceSDK(t)
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := closeSession(); err != nil {
				t.Errorf("close GovernanceInstance cassette: %v", err)
			}
		}
	})

	resource := &accessgovernancecpv1beta1.GovernanceInstance{
		Spec: accessgovernancecpv1beta1.GovernanceInstanceSpec{
			DisplayName:      syntheticGovernanceInstanceName,
			LicenseType:      string(accessgovernancecpsdk.LicenseTypeNewLicense),
			TenancyNamespace: "synthetic-namespace",
			CompartmentId:    "ocid1.compartment.oc1..replay",
			IdcsAccessToken:  "synthetic-administrator-token",
			Description:      "synthetic create",
			FreeformTags: map[string]string{
				"osok-replay": "create",
			},
		},
	}
	client := newGovernanceInstanceServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")},
		sdkClient,
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitSyntheticGovernanceInstanceConvergence(createCtx, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(accessgovernancecpsdk.InstanceLifecycleStateActive) ||
		resource.Status.DisplayName != syntheticGovernanceInstanceName {
		t.Fatalf("created GovernanceInstance status = %+v", resource.Status)
	}

	resource.Spec.Description = "synthetic update"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitSyntheticGovernanceInstanceConvergence(ctx, client, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.Description != resource.Spec.Description ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated GovernanceInstance status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, ocireplay.ModeReplay, 0, func() (bool, error) {
		return client.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func openSyntheticGovernanceInstanceSDK(
	t *testing.T,
) (accessgovernancecpsdk.AccessGovernanceCPClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "accessgovernancecp",
		Resource: "GovernanceInstance",
		Operations: []ocireplay.Operation{
			ocireplay.OperationCreate,
			ocireplay.OperationRead,
			ocireplay.OperationUpdate,
			ocireplay.OperationDelete,
		},
		SDKVersion: "v65.110.0",
		Provenance: ocireplay.ProvenanceSynthetic,
	}
	session, err := ocireplay.OpenSDKReplay(ocireplay.SDKReplayOptions{
		Path: filepath.Join(
			"testdata",
			"recordings",
			"governanceinstance_synthetic_crud.yaml",
		),
		Host:     "https://cp-prod.access-governance.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20220518",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return accessgovernancecpsdk.AccessGovernanceCPClient{
		BaseClient: session.BaseClient(),
	}, session.Close
}

func awaitSyntheticGovernanceInstanceConvergence(
	ctx context.Context,
	client GovernanceInstanceServiceClient,
	resource *accessgovernancecpv1beta1.GovernanceInstance,
) error {
	return ocireplay.Await(ctx, ocireplay.ModeReplay, 0, func() (bool, error) {
		response, err := client.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf(
				"GovernanceInstance reconciliation was unsuccessful: %+v",
				response,
			)
		}
		return !response.ShouldRequeue, nil
	})
}
