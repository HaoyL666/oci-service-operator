/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package metricproperty

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	ocicontrolcentersdk "github.com/oracle/oci-go-sdk/v65/ocicontrolcenter"
	ocicontrolcenterv1beta1 "github.com/oracle/oci-service-operator/api/ocicontrolcenter/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	metricPropertyCompartmentIDAnnotation = "ocicontrolcenter.oracle.com/compartment-id"
	metricPropertyNamespaceNameAnnotation = "ocicontrolcenter.oracle.com/namespace-name"
	metricPropertyMetricNameAnnotation    = "ocicontrolcenter.oracle.com/metric-name"

	metricPropertySyntheticIDPrefix  = "metricproperty/"
	metricPropertySyntheticIDVersion = "v1/"
)

type metricPropertyOCIClient interface {
	ListMetricProperties(context.Context, ocicontrolcentersdk.ListMetricPropertiesRequest) (ocicontrolcentersdk.ListMetricPropertiesResponse, error)
}

type metricPropertySelector struct {
	compartmentID string
	namespaceName string
	metricName    string
}

type metricPropertyRuntimeClient struct {
	client  metricPropertyOCIClient
	log     loggerutil.OSOKLogger
	initErr error
}

var _ MetricPropertyServiceClient = (*metricPropertyRuntimeClient)(nil)

func init() {
	registerMetricPropertyRuntimeHooksMutator(func(manager *MetricPropertyServiceManager, hooks *MetricPropertyRuntimeHooks) {
		applyMetricPropertyRuntimeHooks(hooks)

		client, err := newMetricPropertySDKClient(manager)
		hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(_ MetricPropertyServiceClient) MetricPropertyServiceClient {
			return newMetricPropertyRuntimeClient(manager, client, err)
		})
	})
}

func newMetricPropertySDKClient(manager *MetricPropertyServiceManager) (metricPropertyOCIClient, error) {
	if manager == nil {
		return nil, fmt.Errorf("MetricProperty service manager is nil")
	}
	client, err := ocicontrolcentersdk.NewOccMetricsClientWithConfigurationProvider(manager.Provider)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func applyMetricPropertyRuntimeHooks(hooks *MetricPropertyRuntimeHooks) {
	if hooks == nil {
		return
	}
	hooks.Semantics = reviewedMetricPropertyRuntimeSemantics()
}

func newMetricPropertyRuntimeClient(
	manager *MetricPropertyServiceManager,
	client metricPropertyOCIClient,
	initErr error,
) *metricPropertyRuntimeClient {
	runtimeClient := &metricPropertyRuntimeClient{
		client:  client,
		initErr: initErr,
	}
	if manager != nil {
		runtimeClient.log = manager.Log
	}
	return runtimeClient
}

func reviewedMetricPropertyRuntimeSemantics() *generatedruntime.Semantics {
	return &generatedruntime.Semantics{
		FormalService:     "ocicontrolcenter",
		FormalSlug:        "metricproperty",
		StatusProjection:  "required",
		SecretSideEffects: "none",
		FinalizerPolicy:   "retain-until-confirmed-delete",
		List: &generatedruntime.ListSemantics{
			ResponseItemsField: "Items",
			MatchFields:        []string{"metricName"},
		},
		Mutation: generatedruntime.MutationSemantics{
			Mutable:       []string{},
			ForceNew:      []string{"compartmentId", "namespaceName", "metricName"},
			ConflictsWith: map[string][]string{},
		},
	}
}

func (c *metricPropertyRuntimeClient) CreateOrUpdate(
	ctx context.Context,
	resource *ocicontrolcenterv1beta1.MetricProperty,
	_ ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	if c.initErr != nil {
		return c.failCreateOrUpdate(resource, fmt.Errorf("initialize MetricProperty OCI client: %w", c.initErr))
	}
	if c.client == nil {
		return c.failCreateOrUpdate(resource, fmt.Errorf("MetricProperty OCI client is nil"))
	}

	selector, err := metricPropertySelectorFromResource(resource)
	if err != nil {
		return c.failCreateOrUpdate(resource, err)
	}
	if err := validateMetricPropertyTrackedIdentity(resource, selector); err != nil {
		return c.failCreateOrUpdate(resource, err)
	}

	summary, response, err := c.findMetricProperty(ctx, selector)
	if response.OpcRequestId != nil {
		servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)
	}
	if err != nil {
		return c.failCreateOrUpdate(resource, err)
	}

	projectMetricPropertyStatus(resource, selector, summary)
	markMetricPropertyActive(resource, c.log)
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)
	return servicemanager.OSOKResponse{IsSuccessful: true}, nil
}

func (c *metricPropertyRuntimeClient) Delete(_ context.Context, resource *ocicontrolcenterv1beta1.MetricProperty) (bool, error) {
	if resource == nil {
		return false, fmt.Errorf("MetricProperty resource is nil")
	}
	markMetricPropertyDeleted(resource, "MetricProperty is read-only in OCI; no OCI delete is issued", c.log)
	return true, nil
}

func (c *metricPropertyRuntimeClient) failCreateOrUpdate(
	resource *ocicontrolcenterv1beta1.MetricProperty,
	err error,
) (servicemanager.OSOKResponse, error) {
	if resource != nil {
		markMetricPropertyFailed(resource, err, c.log)
	}
	return servicemanager.OSOKResponse{IsSuccessful: false}, err
}

func (c *metricPropertyRuntimeClient) findMetricProperty(
	ctx context.Context,
	selector metricPropertySelector,
) (ocicontrolcentersdk.MetricPropertySummary, ocicontrolcentersdk.ListMetricPropertiesResponse, error) {
	var (
		page    *string
		last    ocicontrolcentersdk.ListMetricPropertiesResponse
		matches []ocicontrolcentersdk.MetricPropertySummary
	)

	for {
		response, err := c.client.ListMetricProperties(ctx, ocicontrolcentersdk.ListMetricPropertiesRequest{
			NamespaceName: common.String(selector.namespaceName),
			CompartmentId: common.String(selector.compartmentID),
			Page:          page,
		})
		if err != nil {
			return ocicontrolcentersdk.MetricPropertySummary{}, response, err
		}
		last = response

		for _, item := range response.Items {
			if strings.TrimSpace(stringValue(item.MetricName)) == selector.metricName {
				matches = append(matches, item)
			}
		}

		if response.OpcNextPage == nil || strings.TrimSpace(*response.OpcNextPage) == "" {
			break
		}
		nextPage := strings.TrimSpace(*response.OpcNextPage)
		page = &nextPage
	}

	switch len(matches) {
	case 1:
		return matches[0], last, nil
	case 0:
		return ocicontrolcentersdk.MetricPropertySummary{}, last, fmt.Errorf(
			"MetricProperty %q was not found in OCI namespace %q and compartment %q",
			selector.metricName,
			selector.namespaceName,
			selector.compartmentID,
		)
	default:
		return ocicontrolcentersdk.MetricPropertySummary{}, last, fmt.Errorf(
			"MetricProperty list returned multiple entries for metricName %q in OCI namespace %q and compartment %q",
			selector.metricName,
			selector.namespaceName,
			selector.compartmentID,
		)
	}
}

func metricPropertySelectorFromResource(resource *ocicontrolcenterv1beta1.MetricProperty) (metricPropertySelector, error) {
	if resource == nil {
		return metricPropertySelector{}, fmt.Errorf("MetricProperty resource is nil")
	}

	annotations := resource.GetAnnotations()
	selector := metricPropertySelector{
		compartmentID: strings.TrimSpace(annotations[metricPropertyCompartmentIDAnnotation]),
		namespaceName: strings.TrimSpace(annotations[metricPropertyNamespaceNameAnnotation]),
		metricName:    strings.TrimSpace(annotations[metricPropertyMetricNameAnnotation]),
	}
	if selector.metricName == "" {
		selector.metricName = strings.TrimSpace(resource.Name)
	}

	var missing []string
	if selector.compartmentID == "" {
		missing = append(missing, metricPropertyCompartmentIDAnnotation)
	}
	if selector.namespaceName == "" {
		missing = append(missing, metricPropertyNamespaceNameAnnotation)
	}
	if selector.metricName == "" {
		missing = append(missing, metricPropertyMetricNameAnnotation)
	}
	if len(missing) != 0 {
		return metricPropertySelector{}, fmt.Errorf(
			"MetricProperty requires metadata annotation(s) %s because the OCI SDK exposes only ListMetricProperties and the generated CRD spec has no selector fields",
			strings.Join(missing, ", "),
		)
	}

	return selector, nil
}

func validateMetricPropertyTrackedIdentity(
	resource *ocicontrolcenterv1beta1.MetricProperty,
	selector metricPropertySelector,
) error {
	if resource == nil {
		return fmt.Errorf("MetricProperty resource is nil")
	}

	recorded := strings.TrimSpace(string(resource.Status.OsokStatus.Ocid))
	if recorded == "" || !strings.HasPrefix(recorded, metricPropertySyntheticIDPrefix) {
		return nil
	}
	if recorded == string(selector.syntheticOCID()) {
		return nil
	}
	return fmt.Errorf(
		"MetricProperty selector annotations are immutable after binding; delete and recreate the Kubernetes resource to change compartmentId, namespaceName, or metricName",
	)
}

func projectMetricPropertyStatus(
	resource *ocicontrolcenterv1beta1.MetricProperty,
	selector metricPropertySelector,
	summary ocicontrolcentersdk.MetricPropertySummary,
) {
	resource.Status.MetricName = strings.TrimSpace(stringValue(summary.MetricName))
	if resource.Status.MetricName == "" {
		resource.Status.MetricName = selector.metricName
	}
	resource.Status.Dimensions = metricPropertyDimensions(summary.Dimensions)
	resource.Status.OsokStatus.Ocid = selector.syntheticOCID()
}

func metricPropertyDimensions(
	dimensions map[string]ocicontrolcentersdk.DimensionValue,
) map[string]ocicontrolcenterv1beta1.MetricPropertyDimensions {
	if len(dimensions) == 0 {
		return nil
	}
	projected := make(map[string]ocicontrolcenterv1beta1.MetricPropertyDimensions, len(dimensions))
	for key, value := range dimensions {
		projected[key] = ocicontrolcenterv1beta1.MetricPropertyDimensions{
			DimensionValue: stringValue(value.DimensionValue),
		}
	}
	return projected
}

func markMetricPropertyActive(resource *ocicontrolcenterv1beta1.MetricProperty, log loggerutil.OSOKLogger) {
	now := metav1.Now()
	status := &resource.Status.OsokStatus
	if status.CreatedAt == nil {
		status.CreatedAt = &now
	}
	status.UpdatedAt = &now
	status.DeletedAt = nil
	status.Message = "OCI MetricProperty observed"
	status.Reason = string(shared.Active)
	status.Async.Current = nil
	*status = util.UpdateOSOKStatusCondition(*status, shared.Active, corev1.ConditionTrue, "", status.Message, log)
}

func markMetricPropertyFailed(resource *ocicontrolcenterv1beta1.MetricProperty, err error, log loggerutil.OSOKLogger) {
	status := &resource.Status.OsokStatus
	servicemanager.RecordErrorOpcRequestID(status, err)
	now := metav1.Now()
	status.UpdatedAt = &now
	status.Message = err.Error()
	status.Reason = string(shared.Failed)
	if status.Async.Current != nil {
		current := *status.Async.Current
		current.NormalizedClass = shared.OSOKAsyncClassFailed
		current.Message = err.Error()
		current.UpdatedAt = &now
		status.Async.Current = &current
		return
	}
	*status = util.UpdateOSOKStatusCondition(*status, shared.Failed, corev1.ConditionFalse, "", err.Error(), log)
}

func markMetricPropertyDeleted(resource *ocicontrolcenterv1beta1.MetricProperty, message string, log loggerutil.OSOKLogger) {
	now := metav1.Now()
	status := &resource.Status.OsokStatus
	status.DeletedAt = &now
	status.UpdatedAt = &now
	status.Message = message
	status.Reason = string(shared.Terminating)
	status.Async.Current = nil
	*status = util.UpdateOSOKStatusCondition(*status, shared.Terminating, corev1.ConditionTrue, "", message, log)
}

func (selector metricPropertySelector) syntheticOCID() shared.OCID {
	hash := sha256.New()
	for _, value := range []string{selector.compartmentID, selector.namespaceName, selector.metricName} {
		_, _ = hash.Write([]byte(strings.TrimSpace(value)))
		_, _ = hash.Write([]byte{0})
	}
	return shared.OCID(metricPropertySyntheticIDPrefix + metricPropertySyntheticIDVersion + hex.EncodeToString(hash.Sum(nil)))
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
