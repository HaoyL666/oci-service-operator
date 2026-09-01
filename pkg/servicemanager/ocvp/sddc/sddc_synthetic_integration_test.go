/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package sddc

import (
	"context"
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

const syntheticSddcName = "osok-replay-sddc"

// OCI VMware Solution SDDCs allocate multiple billable bare-metal ESXi hosts
// and require a purpose-built network with several VLANs. This synthetic
// cassette exercises the SDK and runtime contract without provisioning that
// infrastructure or claiming that the lifecycle was observed in a live tenancy.
func TestSyntheticSddcCreateUpdateDelete(t *testing.T) {
	sdkClient, closeSession := openSyntheticSddcSDK(t)
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := closeSession(); err != nil {
				t.Errorf("close Sddc cassette: %v", err)
			}
		}
	})

	resource := &ocvpv1beta1.Sddc{Spec: ocvpv1beta1.SddcSpec{
		VmwareSoftwareVersion: "8.0.2",
		CompartmentId:         "ocid1.compartment.oc1..replay",
		HcxMode:               string(ocvpsdk.HcxModesDisabled),
		InitialConfiguration: ocvpv1beta1.SddcInitialConfiguration{
			InitialClusterConfigurations: []ocvpv1beta1.SddcInitialConfigurationInitialClusterConfiguration{
				{
					VsphereType:               string(ocvpsdk.VsphereTypesManagement),
					ComputeAvailabilityDomain: "US-ASHBURN-AD-1",
					EsxiHostsCount:            3,
					DisplayName:               "management",
					InitialCommitment:         string(ocvpsdk.CommitmentHour),
					InitialHostShapeName:      "BM.DenseIO.E5.128",
					InitialHostOcpuCount:      128,
					NetworkConfiguration: ocvpv1beta1.SddcInitialConfigurationInitialClusterConfigurationNetworkConfiguration{
						ProvisioningSubnetId: "ocid1.subnet.oc1..replay",
						VmotionVlanId:        "ocid1.vlan.oc1..vmotion",
						VsanVlanId:           "ocid1.vlan.oc1..vsan",
						NsxVTepVlanId:        "ocid1.vlan.oc1..nsxvtep",
						NsxEdgeVTepVlanId:    "ocid1.vlan.oc1..nsxedgevtep",
						VsphereVlanId:        "ocid1.vlan.oc1..vsphere",
						NsxEdgeUplink1VlanId: "ocid1.vlan.oc1..uplink1",
						NsxEdgeUplink2VlanId: "ocid1.vlan.oc1..uplink2",
					},
				},
			},
		},
		SshAuthorizedKeys: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIReplayOnlyKey osok-replay",
		DisplayName:       syntheticSddcName,
		FreeformTags:      map[string]string{"osok-replay": "create"},
	}}
	manager := newSyntheticSddcManager(sdkClient)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	createCtx := generatedruntime.WithSkipExistingBeforeCreate(ctx)
	if err := awaitSyntheticSddcConvergence(createCtx, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.LifecycleState != string(ocvpsdk.LifecycleStatesActive) ||
		resource.Status.DisplayName != syntheticSddcName ||
		resource.Status.ClustersCount != 1 {
		t.Fatalf("created Sddc status = %+v", resource.Status)
	}

	resource.Spec.DisplayName = syntheticSddcName + "-updated"
	resource.Spec.FreeformTags = map[string]string{"osok-replay": "update"}
	if err := awaitSyntheticSddcConvergence(ctx, manager, resource); err != nil {
		t.Fatal(err)
	}
	if resource.Status.DisplayName != resource.Spec.DisplayName ||
		resource.Status.FreeformTags["osok-replay"] != "update" {
		t.Fatalf("updated Sddc status = %+v", resource.Status)
	}

	if err := ocireplay.Await(ctx, ocireplay.ModeReplay, 0, func() (bool, error) {
		return manager.Delete(ctx, resource)
	}); err != nil {
		t.Fatal(err)
	}

	if err := closeSession(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func newSyntheticSddcManager(sdkClient ocvpsdk.SddcClient) *SddcServiceManager {
	manager := &SddcServiceManager{
		Log: loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("synthetic-integration")},
	}
	hooks := newSddcDefaultRuntimeHooks(sdkClient)
	applySddcRuntimeHooks(&hooks)
	delegate := defaultSddcServiceClient{
		ServiceClient: generatedruntime.NewServiceClient[*ocvpv1beta1.Sddc](
			buildSddcGeneratedRuntimeConfig(manager, hooks),
		),
	}
	return manager.WithClient(wrapSddcGeneratedClient(hooks, delegate))
}

func openSyntheticSddcSDK(t *testing.T) (ocvpsdk.SddcClient, func() error) {
	t.Helper()

	metadata := ocireplay.Metadata{
		Service:  "ocvp",
		Resource: "Sddc",
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
		Path:     filepath.Join("testdata", "recordings", "sddc_synthetic_crud.yaml"),
		Host:     "https://ocvp.us-ashburn-1.oci.oraclecloud.com",
		BasePath: "20230701",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ocvpsdk.SddcClient{BaseClient: session.BaseClient()}, session.Close
}

func awaitSyntheticSddcConvergence(
	ctx context.Context,
	manager *SddcServiceManager,
	resource *ocvpv1beta1.Sddc,
) error {
	return ocireplay.Await(ctx, ocireplay.ModeReplay, 0, func() (bool, error) {
		response, err := manager.CreateOrUpdate(ctx, resource, ctrl.Request{})
		if err != nil {
			return false, err
		}
		if !response.IsSuccessful {
			return false, fmt.Errorf("Sddc reconciliation was unsuccessful: %+v", response)
		}
		return !response.ShouldRequeue, nil
	})
}
