/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package vcn_test

import (
	"context"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	ocicore "github.com/oracle/oci-go-sdk/v65/core"
	corev1beta1 "github.com/oracle/oci-service-operator/api/core/v1beta1"
	mysqlv1beta1 "github.com/oracle/oci-service-operator/api/mysql/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/metrics"
	. "github.com/oracle/oci-service-operator/pkg/servicemanager/core/vcn"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/stretchr/testify/assert"
	ctrl "sigs.k8s.io/controller-runtime"
)

type mockVirtualNetworkClient struct {
	createVcnFn func(context.Context, ocicore.CreateVcnRequest) (ocicore.CreateVcnResponse, error)
	getVcnFn    func(context.Context, ocicore.GetVcnRequest) (ocicore.GetVcnResponse, error)
	listVcnsFn  func(context.Context, ocicore.ListVcnsRequest) (ocicore.ListVcnsResponse, error)
	updateVcnFn func(context.Context, ocicore.UpdateVcnRequest) (ocicore.UpdateVcnResponse, error)
	deleteVcnFn func(context.Context, ocicore.DeleteVcnRequest) (ocicore.DeleteVcnResponse, error)
}

func (m *mockVirtualNetworkClient) CreateVcn(ctx context.Context, req ocicore.CreateVcnRequest) (ocicore.CreateVcnResponse, error) {
	if m.createVcnFn != nil {
		return m.createVcnFn(ctx, req)
	}
	return ocicore.CreateVcnResponse{}, nil
}

func (m *mockVirtualNetworkClient) GetVcn(ctx context.Context, req ocicore.GetVcnRequest) (ocicore.GetVcnResponse, error) {
	if m.getVcnFn != nil {
		return m.getVcnFn(ctx, req)
	}
	return ocicore.GetVcnResponse{}, nil
}

func (m *mockVirtualNetworkClient) ListVcns(ctx context.Context, req ocicore.ListVcnsRequest) (ocicore.ListVcnsResponse, error) {
	if m.listVcnsFn != nil {
		return m.listVcnsFn(ctx, req)
	}
	return ocicore.ListVcnsResponse{}, nil
}

func (m *mockVirtualNetworkClient) UpdateVcn(ctx context.Context, req ocicore.UpdateVcnRequest) (ocicore.UpdateVcnResponse, error) {
	if m.updateVcnFn != nil {
		return m.updateVcnFn(ctx, req)
	}
	return ocicore.UpdateVcnResponse{}, nil
}

func (m *mockVirtualNetworkClient) DeleteVcn(ctx context.Context, req ocicore.DeleteVcnRequest) (ocicore.DeleteVcnResponse, error) {
	if m.deleteVcnFn != nil {
		return m.deleteVcnFn(ctx, req)
	}
	return ocicore.DeleteVcnResponse{}, nil
}

func newTestManager(mockClient *mockVirtualNetworkClient) *VcnServiceManager {
	log := loggerutil.OSOKLogger{Logger: ctrl.Log.WithName("test")}
	m := &metrics.Metrics{Logger: log}
	mgr := NewVcnServiceManager(common.NewRawConfigurationProvider("", "", "", "", "", nil), nil, nil, log, m)
	if mockClient != nil {
		ExportSetClientForTest(mgr, mockClient)
	}
	return mgr
}

func makeVcn(id, displayName string, state ocicore.VcnLifecycleStateEnum) ocicore.Vcn {
	return ocicore.Vcn{
		Id:             common.String(id),
		DisplayName:    common.String(displayName),
		LifecycleState: state,
	}
}

func TestBuildCreateDetailsForTest_GuardsOptionalFieldsAndPrefersCidrBlocks(t *testing.T) {
	resource := corev1beta1.Vcn{
		Spec: corev1beta1.VcnSpec{
			CompartmentId:         "ocid1.compartment.oc1..example",
			CidrBlock:             "10.0.0.0/16",
			CidrBlocks:            []string{"10.0.0.0/16", "10.1.0.0/16"},
			DisplayName:           "example-vcn",
			FreeformTags:          map[string]string{"env": "test"},
			Ipv6PrivateCidrBlocks: []string{"fd00::/56"},
		},
	}

	details, err := ExportBuildCreateDetailsForTest(resource)
	assert.NoError(t, err)
	assert.Nil(t, details.CidrBlock)
	assert.Equal(t, []string{"10.0.0.0/16", "10.1.0.0/16"}, details.CidrBlocks)
	assert.Equal(t, common.String("example-vcn"), details.DisplayName)
	assert.Equal(t, map[string]string{"env": "test"}, details.FreeformTags)
	assert.Equal(t, []string{"fd00::/56"}, details.Ipv6PrivateCidrBlocks)
	assert.Nil(t, details.DnsLabel)
	assert.Nil(t, details.IsIpv6Enabled)
	assert.Nil(t, details.IsOracleGuaAllocationEnabled)
}

func TestCreateOrUpdate_CreateNewProvisioningRequeues(t *testing.T) {
	var captured ocicore.CreateVcnRequest
	mockClient := &mockVirtualNetworkClient{
		listVcnsFn: func(_ context.Context, req ocicore.ListVcnsRequest) (ocicore.ListVcnsResponse, error) {
			assert.Equal(t, common.String("ocid1.compartment.oc1..example"), req.CompartmentId)
			assert.Equal(t, common.String("new-vcn"), req.DisplayName)
			return ocicore.ListVcnsResponse{}, nil
		},
		createVcnFn: func(_ context.Context, req ocicore.CreateVcnRequest) (ocicore.CreateVcnResponse, error) {
			captured = req
			return ocicore.CreateVcnResponse{
				Vcn: makeVcn("ocid1.vcn.oc1..new", "new-vcn", ocicore.VcnLifecycleStateProvisioning),
			}, nil
		},
	}
	mgr := newTestManager(mockClient)

	resource := &corev1beta1.Vcn{
		Spec: corev1beta1.VcnSpec{
			CompartmentId: "ocid1.compartment.oc1..example",
			CidrBlocks:    []string{"10.0.0.0/16"},
			DisplayName:   "new-vcn",
		},
	}

	resp, err := mgr.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	assert.NoError(t, err)
	assert.True(t, resp.IsSuccessful)
	assert.True(t, resp.ShouldRequeue)
	assert.Equal(t, shared.OCID("ocid1.vcn.oc1..new"), resource.Status.OsokStatus.Ocid)
	assert.Equal(t, []string{"10.0.0.0/16"}, captured.CreateVcnDetails.CidrBlocks)
	assert.Nil(t, captured.CreateVcnDetails.CidrBlock)
}

func TestCreateOrUpdate_BindAndUpdateExistingByID(t *testing.T) {
	updateCalled := false
	getCalls := 0
	mockClient := &mockVirtualNetworkClient{
		getVcnFn: func(_ context.Context, req ocicore.GetVcnRequest) (ocicore.GetVcnResponse, error) {
			getCalls++
			if getCalls == 1 {
				return ocicore.GetVcnResponse{
					Vcn: ocicore.Vcn{
						Id:             req.VcnId,
						DisplayName:    common.String("old-vcn"),
						FreeformTags:   map[string]string{"env": "dev"},
						LifecycleState: ocicore.VcnLifecycleStateAvailable,
					},
				}, nil
			}
			return ocicore.GetVcnResponse{
				Vcn: ocicore.Vcn{
					Id:             req.VcnId,
					DisplayName:    common.String("new-vcn"),
					FreeformTags:   map[string]string{"env": "prod"},
					LifecycleState: ocicore.VcnLifecycleStateAvailable,
				},
			}, nil
		},
		updateVcnFn: func(_ context.Context, req ocicore.UpdateVcnRequest) (ocicore.UpdateVcnResponse, error) {
			updateCalled = true
			assert.Equal(t, "ocid1.vcn.oc1..bind", *req.VcnId)
			assert.Equal(t, common.String("new-vcn"), req.UpdateVcnDetails.DisplayName)
			assert.Equal(t, map[string]string{"env": "prod"}, req.UpdateVcnDetails.FreeformTags)
			return ocicore.UpdateVcnResponse{}, nil
		},
	}
	mgr := newTestManager(mockClient)

	resource := &corev1beta1.Vcn{
		Spec: corev1beta1.VcnSpec{
			Id:           "ocid1.vcn.oc1..bind",
			DisplayName:  "new-vcn",
			FreeformTags: map[string]string{"env": "prod"},
		},
	}

	resp, err := mgr.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	assert.NoError(t, err)
	assert.True(t, resp.IsSuccessful)
	assert.False(t, resp.ShouldRequeue)
	assert.True(t, updateCalled)
	assert.Equal(t, shared.Active, resource.Status.OsokStatus.Conditions[len(resource.Status.OsokStatus.Conditions)-1].Type)
}

func TestCreateOrUpdate_FailedLifecycleMarksStatus(t *testing.T) {
	mockClient := &mockVirtualNetworkClient{
		getVcnFn: func(_ context.Context, req ocicore.GetVcnRequest) (ocicore.GetVcnResponse, error) {
			return ocicore.GetVcnResponse{
				Vcn: ocicore.Vcn{
					Id:             req.VcnId,
					DisplayName:    common.String("failed-vcn"),
					LifecycleState: "FAILED",
				},
			}, nil
		},
	}
	mgr := newTestManager(mockClient)

	resource := &corev1beta1.Vcn{
		Spec: corev1beta1.VcnSpec{
			Id: "ocid1.vcn.oc1..failed",
		},
	}

	resp, err := mgr.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	assert.Error(t, err)
	assert.False(t, resp.IsSuccessful)
	assert.Equal(t, shared.Failed, resource.Status.OsokStatus.Conditions[len(resource.Status.OsokStatus.Conditions)-1].Type)
}

func TestGetCrdStatus_WrongType(t *testing.T) {
	mgr := newTestManager(nil)

	_, err := mgr.GetCrdStatus(&mysqlv1beta1.MySqlDbSystem{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to convert the type assertion for Vcn")
}
