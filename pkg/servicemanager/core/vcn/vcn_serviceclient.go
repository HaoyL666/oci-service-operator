/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package vcn

import (
	"context"
	"fmt"
	"reflect"

	"github.com/oracle/oci-go-sdk/v65/common"
	ocicore "github.com/oracle/oci-go-sdk/v65/core"
	corev1beta1 "github.com/oracle/oci-service-operator/api/core/v1beta1"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VirtualNetworkClientInterface defines the OCI operations used by VcnServiceManager.
type VirtualNetworkClientInterface interface {
	CreateVcn(ctx context.Context, request ocicore.CreateVcnRequest) (ocicore.CreateVcnResponse, error)
	GetVcn(ctx context.Context, request ocicore.GetVcnRequest) (ocicore.GetVcnResponse, error)
	ListVcns(ctx context.Context, request ocicore.ListVcnsRequest) (ocicore.ListVcnsResponse, error)
	UpdateVcn(ctx context.Context, request ocicore.UpdateVcnRequest) (ocicore.UpdateVcnResponse, error)
	DeleteVcn(ctx context.Context, request ocicore.DeleteVcnRequest) (ocicore.DeleteVcnResponse, error)
}

func newVirtualNetworkClient(provider common.ConfigurationProvider) (ocicore.VirtualNetworkClient, error) {
	return ocicore.NewVirtualNetworkClientWithConfigurationProvider(provider)
}

// getOCIClient returns the injected client if set, otherwise creates one from the provider.
func (c *VcnServiceManager) getOCIClient() (VirtualNetworkClientInterface, error) {
	if c.ociClient != nil {
		return c.ociClient, nil
	}

	client, err := newVirtualNetworkClient(c.Provider)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (c *VcnServiceManager) CreateVcn(ctx context.Context, vcn corev1beta1.Vcn) (ocicore.CreateVcnResponse, error) {
	client, err := c.getOCIClient()
	if err != nil {
		return ocicore.CreateVcnResponse{}, err
	}

	createDetails, err := buildCreateVcnDetails(vcn)
	if err != nil {
		return ocicore.CreateVcnResponse{}, err
	}

	return client.CreateVcn(ctx, ocicore.CreateVcnRequest{
		CreateVcnDetails: createDetails,
	})
}

func (c *VcnServiceManager) GetVcn(ctx context.Context, vcnID shared.OCID) (*ocicore.Vcn, error) {
	client, err := c.getOCIClient()
	if err != nil {
		return nil, err
	}

	response, err := client.GetVcn(ctx, ocicore.GetVcnRequest{
		VcnId: common.String(string(vcnID)),
	})
	if err != nil {
		return nil, err
	}

	return &response.Vcn, nil
}

func (c *VcnServiceManager) FindVcnByDisplayName(ctx context.Context, vcn corev1beta1.Vcn) (*shared.OCID, error) {
	if vcn.Spec.DisplayName == "" || vcn.Spec.CompartmentId == "" {
		return nil, nil
	}

	client, err := c.getOCIClient()
	if err != nil {
		return nil, err
	}

	response, err := client.ListVcns(ctx, ocicore.ListVcnsRequest{
		CompartmentId: common.String(string(vcn.Spec.CompartmentId)),
		DisplayName:   common.String(vcn.Spec.DisplayName),
		Limit:         common.Int(1),
	})
	if err != nil {
		return nil, err
	}

	if len(response.Items) == 0 || response.Items[0].Id == nil {
		return nil, nil
	}

	vcnID := shared.OCID(*response.Items[0].Id)
	return &vcnID, nil
}

func (c *VcnServiceManager) UpdateVcn(ctx context.Context, desired *corev1beta1.Vcn, existing *ocicore.Vcn) error {
	if desired == nil || existing == nil || existing.Id == nil {
		return nil
	}

	updateDetails := ocicore.UpdateVcnDetails{}
	updateNeeded := false

	if desired.Spec.DisplayName != "" && stringOrEmpty(existing.DisplayName) != desired.Spec.DisplayName {
		updateDetails.DisplayName = common.String(desired.Spec.DisplayName)
		updateNeeded = true
	}

	if desired.Spec.FreeformTags != nil && !reflect.DeepEqual(existing.FreeformTags, desired.Spec.FreeformTags) {
		updateDetails.FreeformTags = desired.Spec.FreeformTags
		updateNeeded = true
	}

	if !updateNeeded {
		return nil
	}

	client, err := c.getOCIClient()
	if err != nil {
		return err
	}

	request := ocicore.UpdateVcnRequest{
		VcnId:            common.String(*existing.Id),
		UpdateVcnDetails: updateDetails,
	}

	if existingDisplayName := stringOrEmpty(existing.DisplayName); existingDisplayName != "" {
		c.Log.DebugLog("Updating VCN", "displayName", existingDisplayName)
	}

	_, err = client.UpdateVcn(ctx, request)
	return err
}

func (c *VcnServiceManager) DeleteVcn(ctx context.Context, vcnID shared.OCID) error {
	client, err := c.getOCIClient()
	if err != nil {
		return err
	}

	_, err = client.DeleteVcn(ctx, ocicore.DeleteVcnRequest{
		VcnId: common.String(string(vcnID)),
	})
	return err
}

func buildCreateVcnDetails(vcn corev1beta1.Vcn) (ocicore.CreateVcnDetails, error) {
	if vcn.Spec.CompartmentId == "" {
		return ocicore.CreateVcnDetails{}, fmt.Errorf("compartmentId is required to create a VCN")
	}

	if vcn.Spec.CidrBlock == "" && len(vcn.Spec.CidrBlocks) == 0 {
		return ocicore.CreateVcnDetails{}, fmt.Errorf("cidrBlock or cidrBlocks is required to create a VCN")
	}

	createDetails := ocicore.CreateVcnDetails{
		CompartmentId: common.String(string(vcn.Spec.CompartmentId)),
	}

	if len(vcn.Spec.CidrBlocks) > 0 {
		createDetails.CidrBlocks = append([]string(nil), vcn.Spec.CidrBlocks...)
	} else if vcn.Spec.CidrBlock != "" {
		createDetails.CidrBlock = common.String(vcn.Spec.CidrBlock)
	}

	if len(vcn.Spec.Ipv6PrivateCidrBlocks) > 0 {
		createDetails.Ipv6PrivateCidrBlocks = append([]string(nil), vcn.Spec.Ipv6PrivateCidrBlocks...)
	}

	if vcn.Spec.IsOracleGuaAllocationEnabled {
		createDetails.IsOracleGuaAllocationEnabled = common.Bool(vcn.Spec.IsOracleGuaAllocationEnabled)
	}

	if vcn.Spec.DisplayName != "" {
		createDetails.DisplayName = common.String(vcn.Spec.DisplayName)
	}

	if vcn.Spec.DnsLabel != "" {
		createDetails.DnsLabel = common.String(vcn.Spec.DnsLabel)
	}

	if vcn.Spec.FreeformTags != nil {
		createDetails.FreeformTags = vcn.Spec.FreeformTags
	}

	if vcn.Spec.IsIpv6Enabled {
		createDetails.IsIpv6Enabled = common.Bool(vcn.Spec.IsIpv6Enabled)
	}

	return createDetails, nil
}

func vcnNeedsUpdate(desired corev1beta1.Vcn, existing ocicore.Vcn) bool {
	return desired.Spec.DisplayName != "" && desired.Spec.DisplayName != stringOrEmpty(existing.DisplayName) ||
		desired.Spec.FreeformTags != nil && !reflect.DeepEqual(desired.Spec.FreeformTags, existing.FreeformTags)
}

func resolveKnownVcnID(vcn *corev1beta1.Vcn) shared.OCID {
	if vcn == nil {
		return ""
	}
	if vcn.Spec.Id != "" {
		return vcn.Spec.Id
	}
	if vcn.Status.OsokStatus.Ocid != "" {
		return vcn.Status.OsokStatus.Ocid
	}
	return ""
}

func populateStatusFromVcn(status *shared.OSOKStatus, vcn ocicore.Vcn) {
	if status == nil {
		return
	}
	if vcn.Id != nil {
		status.Ocid = shared.OCID(*vcn.Id)
	}
	if vcn.TimeCreated != nil {
		created := metav1.NewTime(vcn.TimeCreated.Time)
		status.CreatedAt = &created
	}
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
