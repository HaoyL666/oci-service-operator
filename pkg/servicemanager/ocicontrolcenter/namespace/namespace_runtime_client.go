/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package namespace

import (
	"context"
	"fmt"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	ocicontrolcentersdk "github.com/oracle/oci-go-sdk/v65/ocicontrolcenter"
	ocicontrolcenterv1beta1 "github.com/oracle/oci-service-operator/api/ocicontrolcenter/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const namespaceNameAnnotation = "ocicontrolcenter.oracle.com/namespace-name"

type listNamespacesFunc func(context.Context, ocicontrolcentersdk.ListNamespacesRequest) (ocicontrolcentersdk.ListNamespacesResponse, error)

type namespaceRuntimeClient struct {
	provider       common.ConfigurationProvider
	listNamespaces listNamespacesFunc
	log            loggerutil.OSOKLogger
}

var _ NamespaceServiceClient = (*namespaceRuntimeClient)(nil)

func init() {
	registerNamespaceRuntimeHooksMutator(func(manager *NamespaceServiceManager, hooks *NamespaceRuntimeHooks) {
		if manager == nil || hooks == nil || hooks.List.Call == nil {
			return
		}
		listNamespaces := hooks.List.Call
		hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(delegate NamespaceServiceClient) NamespaceServiceClient {
			if listNamespaces == nil {
				return delegate
			}
			return &namespaceRuntimeClient{
				provider:       manager.Provider,
				listNamespaces: listNamespaces,
				log:            manager.Log,
			}
		})
	})
}

func newNamespaceRuntimeClientForTest(
	provider common.ConfigurationProvider,
	listNamespaces listNamespacesFunc,
) NamespaceServiceClient {
	return &namespaceRuntimeClient{
		provider:       provider,
		listNamespaces: listNamespaces,
		log:            loggerutil.OSOKLogger{},
	}
}

func (c *namespaceRuntimeClient) CreateOrUpdate(
	ctx context.Context,
	resource *ocicontrolcenterv1beta1.Namespace,
	_ ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	targetName, err := namespaceTargetName(resource)
	if err != nil {
		return c.failCreateOrUpdate(resource, err)
	}

	match, err := c.findNamespace(ctx, resource, targetName)
	if err != nil {
		return c.failCreateOrUpdate(resource, err)
	}

	return c.markActive(resource, match), nil
}

func (c *namespaceRuntimeClient) Delete(
	_ context.Context,
	resource *ocicontrolcenterv1beta1.Namespace,
) (bool, error) {
	if resource == nil {
		return false, fmt.Errorf("namespace resource is nil")
	}

	c.markDeleted(resource, "OCI Control Center namespaces are read-only; Kubernetes resource deletion does not delete OCI namespace")
	return true, nil
}

func namespaceTargetName(resource *ocicontrolcenterv1beta1.Namespace) (string, error) {
	if resource == nil {
		return "", fmt.Errorf("namespace resource is nil")
	}

	annotated := strings.TrimSpace(resource.GetAnnotations()[namespaceNameAnnotation])
	recorded := strings.TrimSpace(resource.Status.NamespaceName)
	if annotated != "" && recorded != "" && annotated != recorded {
		return "", fmt.Errorf("namespace metadata annotation %q changes are not supported after binding", namespaceNameAnnotation)
	}
	if annotated != "" {
		return annotated, nil
	}
	if recorded != "" {
		return recorded, nil
	}
	if name := strings.TrimSpace(resource.Name); name != "" {
		return name, nil
	}
	return "", fmt.Errorf("namespace requires metadata.name or metadata annotation %q", namespaceNameAnnotation)
}

func (c *namespaceRuntimeClient) findNamespace(
	ctx context.Context,
	resource *ocicontrolcenterv1beta1.Namespace,
	targetName string,
) (ocicontrolcentersdk.NamespaceSummary, error) {
	compartmentID, err := c.compartmentID()
	if err != nil {
		return ocicontrolcentersdk.NamespaceSummary{}, err
	}
	if c.listNamespaces == nil {
		return ocicontrolcentersdk.NamespaceSummary{}, fmt.Errorf("namespace OCI list client is nil")
	}

	matches, err := c.listNamespaceMatches(ctx, resource, compartmentID, targetName)
	if err != nil {
		return ocicontrolcentersdk.NamespaceSummary{}, err
	}
	return resolveNamespaceMatch(targetName, matches)
}

func (c *namespaceRuntimeClient) listNamespaceMatches(
	ctx context.Context,
	resource *ocicontrolcenterv1beta1.Namespace,
	compartmentID string,
	targetName string,
) ([]ocicontrolcentersdk.NamespaceSummary, error) {
	var (
		matches []ocicontrolcentersdk.NamespaceSummary
		page    *string
	)
	for {
		response, err := c.listNamespaces(ctx, ocicontrolcentersdk.ListNamespacesRequest{
			CompartmentId: common.String(compartmentID),
			Page:          page,
		})
		if err != nil {
			servicemanager.RecordErrorOpcRequestID(&resource.Status.OsokStatus, err)
			return nil, err
		}
		servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)

		for _, item := range response.Items {
			if namespaceStringValue(item.NamespaceName) == targetName {
				matches = append(matches, item)
			}
		}

		if response.OpcNextPage == nil || strings.TrimSpace(*response.OpcNextPage) == "" {
			break
		}
		page = response.OpcNextPage
	}
	return matches, nil
}

func resolveNamespaceMatch(
	targetName string,
	matches []ocicontrolcentersdk.NamespaceSummary,
) (ocicontrolcentersdk.NamespaceSummary, error) {
	switch len(matches) {
	case 0:
		return ocicontrolcentersdk.NamespaceSummary{}, fmt.Errorf("namespace %q was not found in OCI Control Center", targetName)
	case 1:
		return matches[0], nil
	default:
		return ocicontrolcentersdk.NamespaceSummary{}, fmt.Errorf("namespace list returned multiple matches for %q", targetName)
	}
}

func (c *namespaceRuntimeClient) compartmentID() (string, error) {
	if c.provider == nil {
		return "", fmt.Errorf("namespace OCI configuration provider is nil")
	}
	compartmentID, err := c.provider.TenancyOCID()
	if err != nil {
		return "", fmt.Errorf("resolve namespace tenancy OCID: %w", err)
	}
	compartmentID = strings.TrimSpace(compartmentID)
	if compartmentID == "" {
		return "", fmt.Errorf("resolve namespace tenancy OCID: empty tenancy OCID")
	}
	return compartmentID, nil
}

func (c *namespaceRuntimeClient) markActive(
	resource *ocicontrolcenterv1beta1.Namespace,
	current ocicontrolcentersdk.NamespaceSummary,
) servicemanager.OSOKResponse {
	namespaceName := namespaceStringValue(current.NamespaceName)
	resource.Status.NamespaceName = namespaceName

	status := &resource.Status.OsokStatus
	now := metav1.Now()
	if status.CreatedAt == nil {
		status.CreatedAt = &now
	}
	status.UpdatedAt = &now
	status.Message = fmt.Sprintf("observed OCI Control Center namespace %q", namespaceName)
	status.Reason = string(shared.Active)
	resource.Status.OsokStatus = updateNamespaceStatusCondition(
		resource.Status.OsokStatus,
		shared.Active,
		v1.ConditionTrue,
		"",
		status.Message,
		c.log,
	)

	return servicemanager.OSOKResponse{IsSuccessful: true}
}

func (c *namespaceRuntimeClient) failCreateOrUpdate(
	resource *ocicontrolcenterv1beta1.Namespace,
	err error,
) (servicemanager.OSOKResponse, error) {
	c.markFailed(resource, err)
	return servicemanager.OSOKResponse{IsSuccessful: false}, err
}

func (c *namespaceRuntimeClient) markFailed(resource *ocicontrolcenterv1beta1.Namespace, err error) {
	if resource == nil || err == nil {
		return
	}
	status := &resource.Status.OsokStatus
	servicemanager.RecordErrorOpcRequestID(status, err)
	now := metav1.Now()
	status.UpdatedAt = &now
	status.Message = err.Error()
	status.Reason = string(shared.Failed)
	resource.Status.OsokStatus = updateNamespaceStatusCondition(
		resource.Status.OsokStatus,
		shared.Failed,
		v1.ConditionFalse,
		"",
		err.Error(),
		c.log,
	)
}

func (c *namespaceRuntimeClient) markDeleted(resource *ocicontrolcenterv1beta1.Namespace, message string) {
	status := &resource.Status.OsokStatus
	now := metav1.Now()
	status.DeletedAt = &now
	status.UpdatedAt = &now
	status.Message = message
	status.Reason = string(shared.Terminating)
	resource.Status.OsokStatus = updateNamespaceStatusCondition(
		resource.Status.OsokStatus,
		shared.Terminating,
		v1.ConditionTrue,
		"",
		message,
		c.log,
	)
}

func namespaceStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func updateNamespaceStatusCondition(
	osokStatus shared.OSOKStatus,
	conditionType shared.OSOKConditionType,
	status v1.ConditionStatus,
	reason string,
	message string,
	log loggerutil.OSOKLogger,
) shared.OSOKStatus {
	osokStatus = util.UpdateOSOKStatusCondition(osokStatus, conditionType, status, reason, message, log)
	if len(osokStatus.Conditions) == 0 || osokStatus.Conditions[len(osokStatus.Conditions)-1].Type != conditionType {
		now := metav1.Now()
		osokStatus.Conditions = append(osokStatus.Conditions, shared.OSOKCondition{
			Type:               conditionType,
			Status:             status,
			LastTransitionTime: &now,
			Message:            message,
			Reason:             reason,
		})
	}
	return osokStatus
}
