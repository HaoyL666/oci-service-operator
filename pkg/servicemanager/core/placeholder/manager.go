/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package placeholder

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-service-operator/pkg/credhelper"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/metrics"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Manager holds shared dependencies for placeholder core service managers.
type Manager struct {
	Provider         common.ConfigurationProvider
	CredentialClient credhelper.CredentialClient
	Scheme           *runtime.Scheme
	Log              loggerutil.OSOKLogger
	Metrics          *metrics.Metrics
	ResourceName     string
}

// New returns a placeholder manager configured for one core resource kind.
func New(provider common.ConfigurationProvider, credClient credhelper.CredentialClient, scheme *runtime.Scheme,
	log loggerutil.OSOKLogger, metrics *metrics.Metrics, resourceName string) *Manager {
	return &Manager{
		Provider:         provider,
		CredentialClient: credClient,
		Scheme:           scheme,
		Log:              log,
		Metrics:          metrics,
		ResourceName:     resourceName,
	}
}

// MarkNotImplemented updates the CR status with an explicit placeholder failure.
func (m *Manager) MarkNotImplemented(status *shared.OSOKStatus) servicemanager.OSOKResponse {
	message := fmt.Sprintf("%s reconcile logic is not implemented yet", m.ResourceName)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Failed, v1.ConditionFalse, "NotImplemented", message, m.Log)
	status.Message = message
	status.Reason = "NotImplemented"
	return servicemanager.OSOKResponse{IsSuccessful: false}
}

// Delete is a no-op placeholder so finalizers can be removed cleanly until delete logic exists.
func (m *Manager) Delete(context.Context, runtime.Object) (bool, error) {
	return true, nil
}
