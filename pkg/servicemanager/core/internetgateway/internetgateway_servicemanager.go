/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package internetgateway

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	corev1beta1 "github.com/oracle/oci-service-operator/api/core/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/credhelper"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/metrics"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	"github.com/oracle/oci-service-operator/pkg/servicemanager/core/placeholder"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

type InternetGatewayServiceManager struct {
	*placeholder.Manager
}

func NewInternetGatewayServiceManager(provider common.ConfigurationProvider, credClient credhelper.CredentialClient,
	scheme *runtime.Scheme, log loggerutil.OSOKLogger, metrics *metrics.Metrics) *InternetGatewayServiceManager {
	return &InternetGatewayServiceManager{
		Manager: placeholder.New(provider, credClient, scheme, log, metrics, "InternetGateway"),
	}
}

func (m *InternetGatewayServiceManager) CreateOrUpdate(ctx context.Context, obj runtime.Object, req ctrl.Request) (servicemanager.OSOKResponse, error) {
	resource, err := m.convert(obj)
	if err != nil {
		m.Log.ErrorLog(err, "Conversion of object failed", "name", req.Name, "namespace", req.Namespace)
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	return m.MarkNotImplemented(&resource.Status.OsokStatus), nil
}

func (m *InternetGatewayServiceManager) GetCrdStatus(obj runtime.Object) (*shared.OSOKStatus, error) {
	resource, err := m.convert(obj)
	if err != nil {
		return nil, err
	}
	return &resource.Status.OsokStatus, nil
}

func (m *InternetGatewayServiceManager) convert(obj runtime.Object) (*corev1beta1.InternetGateway, error) {
	resource, ok := obj.(*corev1beta1.InternetGateway)
	if !ok {
		return nil, fmt.Errorf("failed to convert the type assertion for InternetGateway")
	}
	return resource, nil
}
