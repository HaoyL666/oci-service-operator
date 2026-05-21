/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package billingschedule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	osubbillingschedulesdk "github.com/oracle/oci-go-sdk/v65/osubbillingschedule"
	osubbillingschedulev1beta1 "github.com/oracle/oci-service-operator/api/osubbillingschedule/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// The generated BillingSchedule CRD has an empty spec and the OCI SDK only
	// exposes ListBillingSchedules, so the resource binds by annotation-backed
	// list identity and records a bounded synthetic identity in shared status.
	billingScheduleCompartmentIDAnnotation       = "osubbillingschedule.oracle.com/compartment-id"
	billingScheduleSubscriptionIDAnnotation      = "osubbillingschedule.oracle.com/subscription-id"
	billingScheduleSubscribedServiceIDAnnotation = "osubbillingschedule.oracle.com/subscribed-service-id"
	billingScheduleOrderNumberAnnotation         = "osubbillingschedule.oracle.com/order-number"
	billingScheduleTimeInvoicingAnnotation       = "osubbillingschedule.oracle.com/time-invoicing"
	billingScheduleProductPartNumberAnnotation   = "osubbillingschedule.oracle.com/product-part-number"
	billingScheduleProductNameAnnotation         = "osubbillingschedule.oracle.com/product-name"
	billingScheduleOriginRegionAnnotation        = "osubbillingschedule.oracle.com/origin-region"

	billingScheduleSyntheticIDPrefix  = "osubbillingschedule/billingschedule/"
	billingScheduleSyntheticIDVersion = "v1/"
	billingScheduleDefaultPageLimit   = 100
	billingScheduleRequeueDuration    = time.Minute
)

type billingScheduleOCIClient interface {
	ListBillingSchedules(context.Context, osubbillingschedulesdk.ListBillingSchedulesRequest) (osubbillingschedulesdk.ListBillingSchedulesResponse, error)
}

type billingScheduleRuntimeClient struct {
	client  billingScheduleOCIClient
	log     loggerutil.OSOKLogger
	initErr error
}

type billingScheduleIdentity struct {
	compartmentID       string
	subscriptionID      string
	subscribedServiceID string
	orderNumber         string
	timeInvoicing       string
	productPartNumber   string
	productName         string
	originRegion        string
}

var _ BillingScheduleServiceClient = (*billingScheduleRuntimeClient)(nil)

func init() {
	registerBillingScheduleRuntimeHooksMutator(func(manager *BillingScheduleServiceManager, hooks *BillingScheduleRuntimeHooks) {
		client, err := newBillingScheduleSDKClient(manager)
		applyBillingScheduleRuntimeHooks(manager, hooks, client, err)
	})
}

func newBillingScheduleSDKClient(manager *BillingScheduleServiceManager) (billingScheduleOCIClient, error) {
	if manager == nil {
		return nil, fmt.Errorf("BillingSchedule service manager is nil")
	}
	client, err := osubbillingschedulesdk.NewBillingScheduleClientWithConfigurationProvider(manager.Provider)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func applyBillingScheduleRuntimeHooks(
	manager *BillingScheduleServiceManager,
	hooks *BillingScheduleRuntimeHooks,
	client billingScheduleOCIClient,
	initErr error,
) {
	if hooks == nil {
		return
	}
	hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(_ BillingScheduleServiceClient) BillingScheduleServiceClient {
		return newBillingScheduleRuntimeClient(manager, client, initErr)
	})
}

func newBillingScheduleRuntimeClient(
	manager *BillingScheduleServiceManager,
	client billingScheduleOCIClient,
	initErr error,
) *billingScheduleRuntimeClient {
	runtimeClient := &billingScheduleRuntimeClient{
		client:  client,
		initErr: initErr,
	}
	if manager != nil {
		runtimeClient.log = manager.Log
	}
	return runtimeClient
}

func newBillingScheduleServiceClientWithOCIClient(
	log loggerutil.OSOKLogger,
	client billingScheduleOCIClient,
) BillingScheduleServiceClient {
	return &billingScheduleRuntimeClient{
		client: client,
		log:    log,
	}
}

func (c *billingScheduleRuntimeClient) CreateOrUpdate(
	ctx context.Context,
	resource *osubbillingschedulev1beta1.BillingSchedule,
	_ ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	if err := c.ensureClient(); err != nil {
		return c.fail(resource, err)
	}
	identity, err := resolveDesiredBillingScheduleIdentity(resource)
	if err != nil {
		return c.fail(resource, err)
	}
	if err := validateTrackedBillingScheduleIdentity(resource, identity); err != nil {
		return c.fail(resource, err)
	}

	summary, response, err := c.readBillingSchedule(ctx, identity)
	if err != nil {
		return c.fail(resource, err)
	}
	projectBillingScheduleStatus(resource, summary)
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)
	return c.markCondition(resource, identity, shared.Active, "BillingSchedule observed from OCI list response", false), nil
}

func (c *billingScheduleRuntimeClient) Delete(
	_ context.Context,
	resource *osubbillingschedulev1beta1.BillingSchedule,
) (bool, error) {
	if resource == nil {
		return false, fmt.Errorf("BillingSchedule resource is nil")
	}
	message := "BillingSchedule is read-only in the OCI SDK; Kubernetes deletion removes OSOK tracking without deleting OCI billing data"
	c.markDeleted(resource, message)
	return true, nil
}

func (c *billingScheduleRuntimeClient) readBillingSchedule(
	ctx context.Context,
	identity billingScheduleIdentity,
) (osubbillingschedulesdk.BillingScheduleSummary, osubbillingschedulesdk.ListBillingSchedulesResponse, error) {
	var matches []osubbillingschedulesdk.BillingScheduleSummary
	var lastResponse osubbillingschedulesdk.ListBillingSchedulesResponse
	page := ""
	for {
		request := buildBillingScheduleListRequest(identity, page)
		response, err := c.client.ListBillingSchedules(ctx, request)
		lastResponse = response
		if err != nil {
			return osubbillingschedulesdk.BillingScheduleSummary{}, response, fmt.Errorf("list BillingSchedules page %q: %w", page, err)
		}
		matches = appendBillingScheduleMatches(matches, response.Items, identity)
		page = strings.TrimSpace(stringValue(response.OpcNextPage))
		if page == "" {
			break
		}
	}

	return singleBillingScheduleMatch(matches, lastResponse, identity)
}

func buildBillingScheduleListRequest(
	identity billingScheduleIdentity,
	page string,
) osubbillingschedulesdk.ListBillingSchedulesRequest {
	request := osubbillingschedulesdk.ListBillingSchedulesRequest{
		CompartmentId:  common.String(identity.compartmentID),
		SubscriptionId: common.String(identity.subscriptionID),
		Limit:          common.Int(billingScheduleDefaultPageLimit),
	}
	if identity.subscribedServiceID != "" {
		request.SubscribedServiceId = common.String(identity.subscribedServiceID)
	}
	if identity.originRegion != "" {
		request.XOneOriginRegion = common.String(identity.originRegion)
	}
	if page != "" {
		request.Page = common.String(page)
	}
	return request
}

func appendBillingScheduleMatches(
	matches []osubbillingschedulesdk.BillingScheduleSummary,
	items []osubbillingschedulesdk.BillingScheduleSummary,
	identity billingScheduleIdentity,
) []osubbillingschedulesdk.BillingScheduleSummary {
	for _, item := range items {
		if billingScheduleMatchesIdentity(item, identity) {
			matches = append(matches, item)
		}
	}
	return matches
}

func singleBillingScheduleMatch(
	matches []osubbillingschedulesdk.BillingScheduleSummary,
	lastResponse osubbillingschedulesdk.ListBillingSchedulesResponse,
	identity billingScheduleIdentity,
) (osubbillingschedulesdk.BillingScheduleSummary, osubbillingschedulesdk.ListBillingSchedulesResponse, error) {
	switch len(matches) {
	case 0:
		return osubbillingschedulesdk.BillingScheduleSummary{}, lastResponse, fmt.Errorf(
			"BillingSchedule not found for %s; the OCI SDK exposes only ListBillingSchedules, so OSOK cannot create a missing billing schedule",
			identity.describe(),
		)
	case 1:
		return matches[0], lastResponse, nil
	default:
		return osubbillingschedulesdk.BillingScheduleSummary{}, lastResponse, fmt.Errorf(
			"BillingSchedule identity %s matched %d OCI list items; add more selector annotations before reconciling",
			identity.describe(),
			len(matches),
		)
	}
}

func billingScheduleMatchesIdentity(
	item osubbillingschedulesdk.BillingScheduleSummary,
	identity billingScheduleIdentity,
) bool {
	return billingScheduleStringMatches(stringValue(item.OrderNumber), identity.orderNumber) &&
		billingScheduleTimeSelectorMatches(item.TimeInvoicing, identity.timeInvoicing) &&
		billingScheduleProductMatchesIdentity(item.Product, identity)
}

func billingScheduleStringMatches(actual string, desired string) bool {
	return desired == "" || strings.TrimSpace(actual) == desired
}

func billingScheduleTimeSelectorMatches(actual *common.SDKTime, desired string) bool {
	return desired == "" || billingScheduleTimeMatches(actual, desired)
}

func billingScheduleProductMatchesIdentity(
	product *osubbillingschedulesdk.Product,
	identity billingScheduleIdentity,
) bool {
	if identity.productPartNumber == "" && identity.productName == "" {
		return true
	}
	if product == nil {
		return false
	}
	return billingScheduleStringMatches(stringValue(product.PartNumber), identity.productPartNumber) &&
		billingScheduleStringMatches(stringValue(product.Name), identity.productName)
}

func billingScheduleTimeMatches(actual *common.SDKTime, desired string) bool {
	if actual == nil {
		return false
	}
	desired = strings.TrimSpace(desired)
	if desired == "" {
		return true
	}
	desiredTime, err := time.Parse(time.RFC3339Nano, desired)
	if err == nil {
		return actual.Equal(desiredTime)
	}
	return sdkTimeString(actual) == desired
}

func projectBillingScheduleStatus(
	resource *osubbillingschedulev1beta1.BillingSchedule,
	summary osubbillingschedulesdk.BillingScheduleSummary,
) {
	if resource == nil {
		return
	}
	resource.Status.TimeStart = sdkTimeString(summary.TimeStart)
	resource.Status.TimeEnd = sdkTimeString(summary.TimeEnd)
	resource.Status.TimeInvoicing = sdkTimeString(summary.TimeInvoicing)
	resource.Status.InvoiceStatus = string(summary.InvoiceStatus)
	resource.Status.Quantity = stringValue(summary.Quantity)
	resource.Status.NetUnitPrice = stringValue(summary.NetUnitPrice)
	resource.Status.Amount = stringValue(summary.Amount)
	resource.Status.BillingFrequency = stringValue(summary.BillingFrequency)
	resource.Status.ArInvoiceNumber = stringValue(summary.ArInvoiceNumber)
	resource.Status.ArCustomerTransactionId = stringValue(summary.ArCustomerTransactionId)
	resource.Status.OrderNumber = stringValue(summary.OrderNumber)
	resource.Status.Product = osubbillingschedulev1beta1.BillingScheduleProduct{}
	if summary.Product != nil {
		resource.Status.Product.PartNumber = stringValue(summary.Product.PartNumber)
		resource.Status.Product.Name = stringValue(summary.Product.Name)
	}
}

func resolveDesiredBillingScheduleIdentity(resource *osubbillingschedulev1beta1.BillingSchedule) (billingScheduleIdentity, error) {
	if resource == nil {
		return billingScheduleIdentity{}, fmt.Errorf("BillingSchedule resource is nil")
	}
	identity := billingScheduleIdentity{
		compartmentID:       annotationValue(resource, billingScheduleCompartmentIDAnnotation),
		subscriptionID:      annotationValue(resource, billingScheduleSubscriptionIDAnnotation),
		subscribedServiceID: annotationValue(resource, billingScheduleSubscribedServiceIDAnnotation),
		orderNumber:         annotationValue(resource, billingScheduleOrderNumberAnnotation),
		timeInvoicing:       normalizeBillingScheduleTime(annotationValue(resource, billingScheduleTimeInvoicingAnnotation)),
		productPartNumber:   annotationValue(resource, billingScheduleProductPartNumberAnnotation),
		productName:         annotationValue(resource, billingScheduleProductNameAnnotation),
		originRegion:        annotationValue(resource, billingScheduleOriginRegionAnnotation),
	}
	return identity, identity.validate()
}

func (identity billingScheduleIdentity) validate() error {
	var missing []string
	if identity.compartmentID == "" {
		missing = append(missing, billingScheduleCompartmentIDAnnotation)
	}
	if identity.subscriptionID == "" {
		missing = append(missing, billingScheduleSubscriptionIDAnnotation)
	}
	if len(missing) > 0 {
		return fmt.Errorf("BillingSchedule requires metadata annotations %s because the generated spec is empty and the OCI SDK only exposes ListBillingSchedules", strings.Join(missing, ", "))
	}
	if identity.subscribedServiceID == "" &&
		identity.orderNumber == "" &&
		identity.timeInvoicing == "" &&
		identity.productPartNumber == "" &&
		identity.productName == "" {
		return fmt.Errorf(
			"BillingSchedule requires at least one selector annotation among %s, %s, %s, %s, or %s",
			billingScheduleSubscribedServiceIDAnnotation,
			billingScheduleOrderNumberAnnotation,
			billingScheduleTimeInvoicingAnnotation,
			billingScheduleProductPartNumberAnnotation,
			billingScheduleProductNameAnnotation,
		)
	}
	return nil
}

func validateTrackedBillingScheduleIdentity(
	resource *osubbillingschedulev1beta1.BillingSchedule,
	desired billingScheduleIdentity,
) error {
	trackedFingerprint, ok := trackedBillingScheduleFingerprint(resource)
	if !ok {
		return nil
	}
	desiredFingerprint := desired.fingerprint()
	if trackedFingerprint == desiredFingerprint {
		return nil
	}
	return fmt.Errorf(
		"BillingSchedule identity is immutable: tracked fingerprint %q, desired identity %s fingerprint %q",
		trackedFingerprint,
		desired.describe(),
		desiredFingerprint,
	)
}

func trackedBillingScheduleFingerprint(resource *osubbillingschedulev1beta1.BillingSchedule) (string, bool) {
	if resource == nil {
		return "", false
	}
	raw := strings.TrimSpace(string(resource.Status.OsokStatus.Ocid))
	prefix := billingScheduleSyntheticIDPrefix + billingScheduleSyntheticIDVersion
	if !strings.HasPrefix(raw, prefix) {
		return "", false
	}
	fingerprint := strings.TrimPrefix(raw, prefix)
	if fingerprint == "" {
		return "", false
	}
	return fingerprint, true
}

func (identity billingScheduleIdentity) syntheticOCID() shared.OCID {
	return shared.OCID(billingScheduleSyntheticIDPrefix + billingScheduleSyntheticIDVersion + identity.fingerprint())
}

func (identity billingScheduleIdentity) fingerprint() string {
	hash := sha256.New()
	for _, value := range []string{
		identity.compartmentID,
		identity.subscriptionID,
		identity.subscribedServiceID,
		identity.orderNumber,
		identity.timeInvoicing,
		identity.productPartNumber,
		identity.productName,
		identity.originRegion,
	} {
		_, _ = hash.Write([]byte(strings.TrimSpace(value)))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (identity billingScheduleIdentity) describe() string {
	parts := []string{
		fmt.Sprintf("%s=%q", billingScheduleCompartmentIDAnnotation, identity.compartmentID),
		fmt.Sprintf("%s=%q", billingScheduleSubscriptionIDAnnotation, identity.subscriptionID),
	}
	if identity.subscribedServiceID != "" {
		parts = append(parts, fmt.Sprintf("%s=%q", billingScheduleSubscribedServiceIDAnnotation, identity.subscribedServiceID))
	}
	if identity.orderNumber != "" {
		parts = append(parts, fmt.Sprintf("%s=%q", billingScheduleOrderNumberAnnotation, identity.orderNumber))
	}
	if identity.timeInvoicing != "" {
		parts = append(parts, fmt.Sprintf("%s=%q", billingScheduleTimeInvoicingAnnotation, identity.timeInvoicing))
	}
	if identity.productPartNumber != "" {
		parts = append(parts, fmt.Sprintf("%s=%q", billingScheduleProductPartNumberAnnotation, identity.productPartNumber))
	}
	if identity.productName != "" {
		parts = append(parts, fmt.Sprintf("%s=%q", billingScheduleProductNameAnnotation, identity.productName))
	}
	if identity.originRegion != "" {
		parts = append(parts, fmt.Sprintf("%s=%q", billingScheduleOriginRegionAnnotation, identity.originRegion))
	}
	return strings.Join(parts, ", ")
}

func (c *billingScheduleRuntimeClient) markCondition(
	resource *osubbillingschedulev1beta1.BillingSchedule,
	identity billingScheduleIdentity,
	condition shared.OSOKConditionType,
	message string,
	shouldRequeue bool,
) servicemanager.OSOKResponse {
	if resource == nil {
		return servicemanager.OSOKResponse{IsSuccessful: condition != shared.Failed}
	}
	status := &resource.Status.OsokStatus
	now := metav1.Now()
	status.Ocid = identity.syntheticOCID()
	if status.CreatedAt == nil {
		status.CreatedAt = &now
	}
	status.UpdatedAt = &now
	status.Message = message
	status.Reason = string(condition)
	if condition == shared.Active {
		servicemanager.ClearAsyncOperation(status)
	}
	conditionStatus := corev1.ConditionTrue
	if condition == shared.Failed {
		conditionStatus = corev1.ConditionFalse
	}
	*status = util.UpdateOSOKStatusCondition(*status, condition, conditionStatus, "", message, c.log)
	return servicemanager.OSOKResponse{
		IsSuccessful:    condition != shared.Failed,
		ShouldRequeue:   shouldRequeue,
		RequeueDuration: billingScheduleRequeueDuration,
	}
}

func (c *billingScheduleRuntimeClient) markDeleted(
	resource *osubbillingschedulev1beta1.BillingSchedule,
	message string,
) {
	if resource == nil {
		return
	}
	now := metav1.Now()
	status := &resource.Status.OsokStatus
	status.DeletedAt = &now
	status.UpdatedAt = &now
	status.Message = message
	status.Reason = string(shared.Terminating)
	servicemanager.ClearAsyncOperation(status)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Terminating, corev1.ConditionTrue, "", message, c.log)
}

func (c *billingScheduleRuntimeClient) fail(
	resource *osubbillingschedulev1beta1.BillingSchedule,
	err error,
) (servicemanager.OSOKResponse, error) {
	if resource != nil && err != nil {
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
			_ = servicemanager.ApplyAsyncOperation(status, &current, c.log)
			return servicemanager.OSOKResponse{IsSuccessful: false}, err
		}
		*status = util.UpdateOSOKStatusCondition(*status, shared.Failed, corev1.ConditionFalse, "", err.Error(), c.log)
	}
	return servicemanager.OSOKResponse{IsSuccessful: false}, err
}

func (c *billingScheduleRuntimeClient) ensureClient() error {
	if c.initErr != nil {
		return fmt.Errorf("initialize BillingSchedule OCI client: %w", c.initErr)
	}
	if c.client == nil {
		return errors.New("BillingSchedule OCI client is not configured")
	}
	return nil
}

func annotationValue(resource *osubbillingschedulev1beta1.BillingSchedule, name string) string {
	if resource == nil || resource.Annotations == nil {
		return ""
	}
	return strings.TrimSpace(resource.Annotations[name])
}

func normalizeBillingScheduleTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return parsed.UTC().Format(time.RFC3339Nano)
}

func sdkTimeString(value *common.SDKTime) string {
	if value == nil {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339Nano)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
