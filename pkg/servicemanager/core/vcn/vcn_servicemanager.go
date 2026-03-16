/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package vcn

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	ocicore "github.com/oracle/oci-go-sdk/v65/core"
	corev1beta1 "github.com/oracle/oci-service-operator/api/core/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/credhelper"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/metrics"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const defaultVcnRequeueDuration = 30 * time.Second
const vcnMetricsComponent = "Vcn"

type VcnServiceManager struct {
	Provider         common.ConfigurationProvider
	CredentialClient credhelper.CredentialClient
	Scheme           *runtime.Scheme
	Log              loggerutil.OSOKLogger
	Metrics          *metrics.Metrics
	ociClient        VirtualNetworkClientInterface
}

func NewVcnServiceManager(provider common.ConfigurationProvider, credClient credhelper.CredentialClient,
	scheme *runtime.Scheme, log loggerutil.OSOKLogger, metrics *metrics.Metrics) *VcnServiceManager {
	return &VcnServiceManager{
		Provider:         provider,
		CredentialClient: credClient,
		Scheme:           scheme,
		Log:              log,
		Metrics:          metrics,
	}
}

func (c *VcnServiceManager) CreateOrUpdate(ctx context.Context, obj runtime.Object, req ctrl.Request) (servicemanager.OSOKResponse, error) {
	vcn, err := c.convert(obj)
	if err != nil {
		c.Log.ErrorLog(err, "Conversion of object failed")
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}

	existing, err := c.resolveExistingVcn(ctx, vcn)
	if err != nil {
		c.markFault(ctx, vcn, req, "Error while resolving VCN", err)
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}

	if existing == nil {
		response, err := c.CreateVcn(ctx, *vcn)
		if err != nil {
			c.markFault(ctx, vcn, req, "Failed to create VCN", err)
			return servicemanager.OSOKResponse{IsSuccessful: false}, err
		}

		existing = &response.Vcn
		vcn.Status.OsokStatus = util.UpdateOSOKStatusCondition(vcn.Status.OsokStatus,
			shared.Provisioning, v1.ConditionTrue, "", "VCN is provisioning", c.Log)
	}

	if vcnNeedsUpdate(*vcn, *existing) {
		if err := c.UpdateVcn(ctx, vcn, existing); err != nil {
			c.markFault(ctx, vcn, req, "Failed to update VCN", err)
			return servicemanager.OSOKResponse{IsSuccessful: false}, err
		}

		if existing.Id == nil {
			err := fmt.Errorf("existing VCN missing id after update")
			c.markFault(ctx, vcn, req, "Failed to refresh VCN after update", err)
			return servicemanager.OSOKResponse{IsSuccessful: false}, err
		}

		refreshed, err := c.GetVcn(ctx, shared.OCID(*existing.Id))
		if err != nil {
			c.markFault(ctx, vcn, req, "Failed to refresh VCN after update", err)
			return servicemanager.OSOKResponse{IsSuccessful: false}, err
		}
		existing = refreshed
		vcn.Status.OsokStatus = util.UpdateOSOKStatusCondition(vcn.Status.OsokStatus,
			shared.Updating, v1.ConditionTrue, "", "VCN update submitted", c.Log)
	}

	populateStatusFromVcn(&vcn.Status.OsokStatus, *existing)
	return c.syncLifecycleStatus(ctx, vcn, existing, req)
}

func (c *VcnServiceManager) Delete(ctx context.Context, obj runtime.Object) (bool, error) {
	vcn, err := c.convert(obj)
	if err != nil {
		c.Log.ErrorLog(err, "Error while converting the object")
		return true, nil
	}

	existing, err := c.resolveExistingVcn(ctx, vcn)
	if err != nil {
		if isOCIResourceNotFound(err) {
			return true, nil
		}
		return false, err
	}
	if existing == nil || existing.Id == nil {
		return true, nil
	}

	if err := c.DeleteVcn(ctx, shared.OCID(*existing.Id)); err != nil && !isOCIResourceNotFound(err) {
		return false, err
	}

	current, err := c.GetVcn(ctx, shared.OCID(*existing.Id))
	if err != nil {
		if isOCIResourceNotFound(err) {
			return true, nil
		}
		return false, err
	}

	switch current.LifecycleState {
	case ocicore.VcnLifecycleStateTerminated:
		return true, nil
	case ocicore.VcnLifecycleStateTerminating:
		return false, nil
	default:
		return false, nil
	}
}

func (c *VcnServiceManager) GetCrdStatus(obj runtime.Object) (*shared.OSOKStatus, error) {
	resource, err := c.convert(obj)
	if err != nil {
		return nil, err
	}
	return &resource.Status.OsokStatus, nil
}

func (c *VcnServiceManager) convert(obj runtime.Object) (*corev1beta1.Vcn, error) {
	resource, ok := obj.(*corev1beta1.Vcn)
	if !ok {
		return nil, fmt.Errorf("failed to convert the type assertion for Vcn")
	}
	return resource, nil
}

func (c *VcnServiceManager) resolveExistingVcn(ctx context.Context, vcn *corev1beta1.Vcn) (*ocicore.Vcn, error) {
	if knownID := resolveKnownVcnID(vcn); knownID != "" {
		existing, err := c.GetVcn(ctx, knownID)
		if err == nil {
			return existing, nil
		}
		if !(isOCIResourceNotFound(err) && vcn.Spec.Id == "") {
			return nil, err
		}
	}

	foundID, err := c.FindVcnByDisplayName(ctx, *vcn)
	if err != nil || foundID == nil {
		return nil, err
	}

	return c.GetVcn(ctx, *foundID)
}

func (c *VcnServiceManager) syncLifecycleStatus(ctx context.Context, vcn *corev1beta1.Vcn, existing *ocicore.Vcn, req ctrl.Request) (servicemanager.OSOKResponse, error) {
	displayName := stringOrEmpty(existing.DisplayName)
	if displayName == "" {
		displayName = req.Name
	}

	switch existing.LifecycleState {
	case "FAILED":
		message := fmt.Sprintf("VCN %s creation failed", displayName)
		vcn.Status.OsokStatus = util.UpdateOSOKStatusCondition(vcn.Status.OsokStatus,
			shared.Failed, v1.ConditionFalse, "", message, c.Log)
		if c.Metrics != nil {
			c.Metrics.AddCRFaultMetrics(ctx, vcnMetricsComponent, message, req.Name, req.Namespace)
		}
		return servicemanager.OSOKResponse{IsSuccessful: false}, errors.New(message)
	case ocicore.VcnLifecycleStateAvailable:
		message := fmt.Sprintf("VCN %s is available", displayName)
		vcn.Status.OsokStatus = util.UpdateOSOKStatusCondition(vcn.Status.OsokStatus,
			shared.Active, v1.ConditionTrue, "", message, c.Log)
		if c.Metrics != nil {
			c.Metrics.AddCRSuccessMetrics(ctx, vcnMetricsComponent, message, req.Name, req.Namespace)
		}
		return servicemanager.OSOKResponse{IsSuccessful: true}, nil
	default:
		message := fmt.Sprintf("VCN %s is %s", displayName, existing.LifecycleState)
		vcn.Status.OsokStatus = util.UpdateOSOKStatusCondition(vcn.Status.OsokStatus,
			shared.Provisioning, v1.ConditionTrue, "", message, c.Log)
		return servicemanager.OSOKResponse{
			IsSuccessful:    true,
			ShouldRequeue:   true,
			RequeueDuration: defaultVcnRequeueDuration,
		}, nil
	}
}

func (c *VcnServiceManager) markFault(ctx context.Context, vcn *corev1beta1.Vcn, req ctrl.Request, message string, err error) {
	vcn.Status.OsokStatus = util.UpdateOSOKStatusCondition(vcn.Status.OsokStatus,
		shared.Failed, v1.ConditionFalse, "", err.Error(), c.Log)
	if c.Metrics != nil {
		c.Metrics.AddCRFaultMetrics(ctx, vcnMetricsComponent, message, req.Name, req.Namespace)
	}
}

func isOCIResourceNotFound(err error) bool {
	if err == nil {
		return false
	}

	serviceErr, ok := err.(common.ServiceError)
	if !ok {
		return false
	}

	return serviceErr.GetHTTPStatusCode() == 404
}
