/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package javamigrationanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	jmsutilssdk "github.com/oracle/oci-go-sdk/v65/jmsutils"
	jmsutilsv1beta1 "github.com/oracle/oci-service-operator/api/jmsutils/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	javaMigrationAnalysisPollInterval = 30 * time.Second
	javaMigrationAnalysisIDAnnotation = "jmsutils.oracle.com/javamigrationanalysis-id"
)

type javaMigrationAnalysisOCIClient interface {
	RequestJavaMigrationAnalysis(context.Context, jmsutilssdk.RequestJavaMigrationAnalysisRequest) (jmsutilssdk.RequestJavaMigrationAnalysisResponse, error)
	GetJavaMigrationAnalysis(context.Context, jmsutilssdk.GetJavaMigrationAnalysisRequest) (jmsutilssdk.GetJavaMigrationAnalysisResponse, error)
	ListJavaMigrationAnalysis(context.Context, jmsutilssdk.ListJavaMigrationAnalysisRequest) (jmsutilssdk.ListJavaMigrationAnalysisResponse, error)
	DeleteJavaMigrationAnalysis(context.Context, jmsutilssdk.DeleteJavaMigrationAnalysisRequest) (jmsutilssdk.DeleteJavaMigrationAnalysisResponse, error)
	GetWorkRequest(context.Context, jmsutilssdk.GetWorkRequestRequest) (jmsutilssdk.GetWorkRequestResponse, error)
}

type javaMigrationAnalysisRuntimeClient struct {
	client  javaMigrationAnalysisOCIClient
	log     loggerutil.OSOKLogger
	initErr error
}

var _ JavaMigrationAnalysisServiceClient = (*javaMigrationAnalysisRuntimeClient)(nil)

func init() {
	registerJavaMigrationAnalysisRuntimeHooksMutator(func(manager *JavaMigrationAnalysisServiceManager, hooks *JavaMigrationAnalysisRuntimeHooks) {
		if manager == nil || hooks == nil {
			return
		}

		ociClient, err := jmsutilssdk.NewJmsUtilsClientWithConfigurationProvider(manager.Provider)
		hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(_ JavaMigrationAnalysisServiceClient) JavaMigrationAnalysisServiceClient {
			return &javaMigrationAnalysisRuntimeClient{
				client:  ociClient,
				log:     manager.Log,
				initErr: err,
			}
		})
	})
}

func newJavaMigrationAnalysisRuntimeClientForTest(client javaMigrationAnalysisOCIClient) JavaMigrationAnalysisServiceClient {
	return &javaMigrationAnalysisRuntimeClient{
		client: client,
	}
}

func (c *javaMigrationAnalysisRuntimeClient) CreateOrUpdate(
	ctx context.Context,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
	_ ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	if err := c.ready(resource); err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	if err := c.rejectIdentityDrift(resource); err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	if hasJavaMigrationAnalysisDesiredSpec(resource) {
		if err := c.validateDesiredSpec(resource); err != nil {
			return servicemanager.OSOKResponse{IsSuccessful: false}, err
		}
	}

	if id := currentJavaMigrationAnalysisID(resource); id != "" {
		return c.observeByID(ctx, resource, id)
	}

	if workRequestID := currentJavaMigrationAnalysisWorkRequestID(resource); workRequestID != "" {
		return c.observeWorkRequest(ctx, resource, workRequestID)
	}

	if hasJavaMigrationAnalysisDesiredSpec(resource) {
		return c.bindBySpec(ctx, resource, true)
	}

	if canBindJavaMigrationAnalysisFromStatus(resource) {
		return c.bindByStatus(ctx, resource)
	}

	err := fmt.Errorf("JavaMigrationAnalysis spec.compartmentId, spec.analysisProjectName, spec.inputApplicationsObjectStoragePaths, and spec.targetJdkVersion are required to request creation")
	c.markFailed(resource, err.Error())
	return servicemanager.OSOKResponse{IsSuccessful: false}, err
}

func (c *javaMigrationAnalysisRuntimeClient) Delete(
	ctx context.Context,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
) (bool, error) {
	if err := c.ready(resource); err != nil {
		return false, err
	}

	id := currentJavaMigrationAnalysisID(resource)
	if id == "" {
		c.markDeleted(resource, "JavaMigrationAnalysis has no recorded OCI identity; finalizer can be removed")
		return true, nil
	}

	if javaMigrationAnalysisDeletePending(resource) {
		return c.confirmDelete(ctx, resource, id)
	}

	current, err := c.client.GetJavaMigrationAnalysis(ctx, jmsutilssdk.GetJavaMigrationAnalysisRequest{
		JavaMigrationAnalysisId: &id,
	})
	if err != nil {
		servicemanager.RecordErrorOpcRequestID(&resource.Status.OsokStatus, err)
		if javaMigrationAnalysisIsUnambiguousNotFound(err) {
			c.markDeleted(resource, fmt.Sprintf("JavaMigrationAnalysis %s is no longer present in OCI", id))
			return true, nil
		}
		if javaMigrationAnalysisIsAuthShapedNotFound(err) {
			return false, c.rejectAmbiguousDelete(resource, "pre-delete read", err)
		}
		c.markFailed(resource, err.Error())
		return false, err
	}
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, current)
	c.projectStatus(resource, current.JavaMigrationAnalysis)

	deleteRequest := jmsutilssdk.DeleteJavaMigrationAnalysisRequest{
		JavaMigrationAnalysisId: &id,
		IfMatch:                 current.Etag,
	}
	deleteResponse, err := c.client.DeleteJavaMigrationAnalysis(ctx, deleteRequest)
	if err != nil {
		servicemanager.RecordErrorOpcRequestID(&resource.Status.OsokStatus, err)
		if javaMigrationAnalysisIsUnambiguousNotFound(err) {
			c.markDeleted(resource, fmt.Sprintf("JavaMigrationAnalysis %s delete returned not found", id))
			return true, nil
		}
		if javaMigrationAnalysisIsAuthShapedNotFound(err) {
			return false, c.rejectAmbiguousDelete(resource, "delete", err)
		}
		c.markFailed(resource, err.Error())
		return false, err
	}
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, deleteResponse)

	return c.confirmDelete(ctx, resource, id)
}

func (c *javaMigrationAnalysisRuntimeClient) confirmDelete(
	ctx context.Context,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
	id string,
) (bool, error) {
	confirm, err := c.client.GetJavaMigrationAnalysis(ctx, jmsutilssdk.GetJavaMigrationAnalysisRequest{
		JavaMigrationAnalysisId: &id,
	})
	if err != nil {
		servicemanager.RecordErrorOpcRequestID(&resource.Status.OsokStatus, err)
		if javaMigrationAnalysisIsUnambiguousNotFound(err) {
			c.markDeleted(resource, fmt.Sprintf("JavaMigrationAnalysis %s deletion is confirmed", id))
			return true, nil
		}
		if javaMigrationAnalysisIsAuthShapedNotFound(err) {
			return false, c.rejectAmbiguousDelete(resource, "post-delete confirmation read", err)
		}
		c.markFailed(resource, err.Error())
		return false, err
	}
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, confirm)
	c.projectStatus(resource, confirm.JavaMigrationAnalysis)
	c.markTerminating(resource, fmt.Sprintf("JavaMigrationAnalysis %s deletion is pending; OCI readback still returns the resource", id))
	return false, nil
}

func (c *javaMigrationAnalysisRuntimeClient) ready(resource *jmsutilsv1beta1.JavaMigrationAnalysis) error {
	if resource == nil {
		return fmt.Errorf("JavaMigrationAnalysis resource is nil")
	}
	if c.initErr != nil {
		err := fmt.Errorf("initialize JavaMigrationAnalysis OCI client: %w", c.initErr)
		c.markFailed(resource, err.Error())
		return err
	}
	if c.client == nil {
		err := fmt.Errorf("JavaMigrationAnalysis OCI client is not configured")
		c.markFailed(resource, err.Error())
		return err
	}
	return nil
}

func (c *javaMigrationAnalysisRuntimeClient) rejectIdentityDrift(resource *jmsutilsv1beta1.JavaMigrationAnalysis) error {
	recordedID := recordedJavaMigrationAnalysisID(resource)
	annotationID := javaMigrationAnalysisAnnotationID(resource)
	if recordedID == "" || annotationID == "" || recordedID == annotationID {
		return nil
	}
	err := fmt.Errorf("JavaMigrationAnalysis metadata annotation %q is create-only and does not match recorded OCI identity %q", javaMigrationAnalysisIDAnnotation, recordedID)
	c.markFailed(resource, err.Error())
	return err
}

func (c *javaMigrationAnalysisRuntimeClient) observeByID(
	ctx context.Context,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
	id string,
) (servicemanager.OSOKResponse, error) {
	response, err := c.client.GetJavaMigrationAnalysis(ctx, jmsutilssdk.GetJavaMigrationAnalysisRequest{
		JavaMigrationAnalysisId: &id,
	})
	if err != nil {
		servicemanager.RecordErrorOpcRequestID(&resource.Status.OsokStatus, err)
		c.markFailed(resource, err.Error())
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)
	c.projectStatus(resource, response.JavaMigrationAnalysis)
	if err := c.rejectCreateOnlyDrift(resource, response.JavaMigrationAnalysis); err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	c.markActive(resource, fmt.Sprintf("JavaMigrationAnalysis %s observed successfully", stringValue(response.Id)))
	return servicemanager.OSOKResponse{IsSuccessful: true}, nil
}

func (c *javaMigrationAnalysisRuntimeClient) observeWorkRequest(
	ctx context.Context,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
	workRequestID string,
) (servicemanager.OSOKResponse, error) {
	response, err := c.client.GetWorkRequest(ctx, jmsutilssdk.GetWorkRequestRequest{
		WorkRequestId: &workRequestID,
	})
	if err != nil {
		servicemanager.RecordErrorOpcRequestID(&resource.Status.OsokStatus, err)
		c.markFailed(resource, err.Error())
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)
	resource.Status.WorkRequestId = workRequestID

	class := javaMigrationAnalysisWorkRequestClass(response.Status)
	c.projectWorkRequestStatus(resource, response.WorkRequest, shared.OSOKAsyncPhaseCreate, class)
	switch class {
	case shared.OSOKAsyncClassPending, shared.OSOKAsyncClassUnknown:
		return servicemanager.OSOKResponse{
			IsSuccessful:    true,
			ShouldRequeue:   true,
			RequeueDuration: javaMigrationAnalysisPollInterval,
		}, nil
	case shared.OSOKAsyncClassSucceeded:
		id := javaMigrationAnalysisIDFromWorkRequest(response.WorkRequest)
		if id == "" {
			if hasJavaMigrationAnalysisDesiredSpec(resource) {
				return c.bindBySpec(ctx, resource, false)
			}
			err := fmt.Errorf("JavaMigrationAnalysis work request %s succeeded without a resource identifier", workRequestID)
			c.markFailed(resource, err.Error())
			return servicemanager.OSOKResponse{IsSuccessful: false}, err
		}
		return c.observeByID(ctx, resource, id)
	case shared.OSOKAsyncClassFailed, shared.OSOKAsyncClassCanceled, shared.OSOKAsyncClassAttention:
		err := fmt.Errorf("JavaMigrationAnalysis work request %s finished with status %s", workRequestID, response.Status)
		c.markFailed(resource, err.Error())
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	default:
		return servicemanager.OSOKResponse{
			IsSuccessful:    true,
			ShouldRequeue:   true,
			RequeueDuration: javaMigrationAnalysisPollInterval,
		}, nil
	}
}

func (c *javaMigrationAnalysisRuntimeClient) bindBySpec(
	ctx context.Context,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
	allowCreate bool,
) (servicemanager.OSOKResponse, error) {
	matches, err := c.listMatchingSpec(ctx, resource)
	if err != nil {
		servicemanager.RecordErrorOpcRequestID(&resource.Status.OsokStatus, err)
		c.markFailed(resource, err.Error())
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	switch len(matches) {
	case 0:
		if allowCreate {
			return c.requestCreate(ctx, resource)
		}
		err := fmt.Errorf("JavaMigrationAnalysis work request completed but no analysis matched spec.compartmentId=%q, spec.analysisProjectName=%q, and spec.targetJdkVersion=%q",
			resource.Spec.CompartmentId, resource.Spec.AnalysisProjectName, resource.Spec.TargetJdkVersion)
		c.markFailed(resource, err.Error())
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	case 1:
		id := stringValue(matches[0].Id)
		if id == "" {
			err := fmt.Errorf("matched JavaMigrationAnalysis summary has empty id")
			c.markFailed(resource, err.Error())
			return servicemanager.OSOKResponse{IsSuccessful: false}, err
		}
		return c.observeByID(ctx, resource, id)
	default:
		err := fmt.Errorf("multiple JavaMigrationAnalysis resources matched spec.compartmentId=%q, spec.analysisProjectName=%q, and spec.targetJdkVersion=%q",
			resource.Spec.CompartmentId, resource.Spec.AnalysisProjectName, resource.Spec.TargetJdkVersion)
		c.markFailed(resource, err.Error())
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
}

func (c *javaMigrationAnalysisRuntimeClient) requestCreate(
	ctx context.Context,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
) (servicemanager.OSOKResponse, error) {
	request := jmsutilssdk.RequestJavaMigrationAnalysisRequest{
		RequestJavaMigrationAnalysisDetails: javaMigrationAnalysisCreateDetails(resource),
		OpcRetryToken:                       common.String(javaMigrationAnalysisRetryToken(resource)),
	}
	response, err := c.client.RequestJavaMigrationAnalysis(ctx, request)
	if err != nil {
		servicemanager.RecordErrorOpcRequestID(&resource.Status.OsokStatus, err)
		c.markFailed(resource, err.Error())
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)

	workRequestID := stringValue(response.OpcWorkRequestId)
	if workRequestID == "" {
		err := fmt.Errorf("RequestJavaMigrationAnalysis response did not include opc-work-request-id")
		c.markFailed(resource, err.Error())
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	resource.Status.WorkRequestId = workRequestID
	c.projectWorkRequestStatus(resource, jmsutilssdk.WorkRequest{
		Id:            common.String(workRequestID),
		Status:        jmsutilssdk.OperationStatusAccepted,
		OperationType: jmsutilssdk.OperationTypeRequestJavaMigrationSaAnalysis,
		CompartmentId: common.String(strings.TrimSpace(resource.Spec.CompartmentId)),
	}, shared.OSOKAsyncPhaseCreate, shared.OSOKAsyncClassPending)
	return servicemanager.OSOKResponse{
		IsSuccessful:    true,
		ShouldRequeue:   true,
		RequeueDuration: javaMigrationAnalysisPollInterval,
	}, nil
}

func (c *javaMigrationAnalysisRuntimeClient) listMatchingSpec(
	ctx context.Context,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
) ([]jmsutilssdk.JavaMigrationAnalysisSummary, error) {
	compartmentID := strings.TrimSpace(resource.Spec.CompartmentId)
	projectName := strings.TrimSpace(resource.Spec.AnalysisProjectName)
	request := jmsutilssdk.ListJavaMigrationAnalysisRequest{
		CompartmentId:       &compartmentID,
		AnalysisProjectName: &projectName,
		SortBy:              jmsutilssdk.ListJavaMigrationAnalysisSortByCreated,
		SortOrder:           jmsutilssdk.ListJavaMigrationAnalysisSortOrderDesc,
	}

	seenPages := map[string]struct{}{}
	var matches []jmsutilssdk.JavaMigrationAnalysisSummary
	for {
		response, err := c.client.ListJavaMigrationAnalysis(ctx, request)
		if err != nil {
			return nil, err
		}
		for _, item := range response.Items {
			if javaMigrationAnalysisSummaryMatchesSpec(item, resource) {
				matches = append(matches, item)
			}
		}
		nextPage := stringValue(response.OpcNextPage)
		if nextPage == "" {
			return matches, nil
		}
		if _, ok := seenPages[nextPage]; ok {
			return nil, fmt.Errorf("JavaMigrationAnalysis list pagination repeated page token %q", nextPage)
		}
		seenPages[nextPage] = struct{}{}
		request.Page = response.OpcNextPage
	}
}

func (c *javaMigrationAnalysisRuntimeClient) bindByStatus(
	ctx context.Context,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
) (servicemanager.OSOKResponse, error) {
	matches, err := c.listMatchingStatus(ctx, resource)
	if err != nil {
		servicemanager.RecordErrorOpcRequestID(&resource.Status.OsokStatus, err)
		c.markFailed(resource, err.Error())
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
	switch len(matches) {
	case 0:
		err := fmt.Errorf("no JavaMigrationAnalysis matched status.compartmentId=%q and status.analysisProjectName=%q",
			resource.Status.CompartmentId, resource.Status.AnalysisProjectName)
		c.markFailed(resource, err.Error())
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	case 1:
		id := stringValue(matches[0].Id)
		if id == "" {
			err := fmt.Errorf("matched JavaMigrationAnalysis summary has empty id")
			c.markFailed(resource, err.Error())
			return servicemanager.OSOKResponse{IsSuccessful: false}, err
		}
		return c.observeByID(ctx, resource, id)
	default:
		err := fmt.Errorf("multiple JavaMigrationAnalysis resources matched status.compartmentId=%q and status.analysisProjectName=%q",
			resource.Status.CompartmentId, resource.Status.AnalysisProjectName)
		c.markFailed(resource, err.Error())
		return servicemanager.OSOKResponse{IsSuccessful: false}, err
	}
}

func (c *javaMigrationAnalysisRuntimeClient) listMatchingStatus(
	ctx context.Context,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
) ([]jmsutilssdk.JavaMigrationAnalysisSummary, error) {
	compartmentID := strings.TrimSpace(resource.Status.CompartmentId)
	projectName := strings.TrimSpace(resource.Status.AnalysisProjectName)
	request := jmsutilssdk.ListJavaMigrationAnalysisRequest{
		CompartmentId:       &compartmentID,
		AnalysisProjectName: &projectName,
		SortBy:              jmsutilssdk.ListJavaMigrationAnalysisSortByCreated,
		SortOrder:           jmsutilssdk.ListJavaMigrationAnalysisSortOrderDesc,
	}

	seenPages := map[string]struct{}{}
	var matches []jmsutilssdk.JavaMigrationAnalysisSummary
	for {
		response, err := c.client.ListJavaMigrationAnalysis(ctx, request)
		if err != nil {
			return nil, err
		}
		for _, item := range response.Items {
			if javaMigrationAnalysisSummaryMatchesStatus(item, compartmentID, projectName) {
				matches = append(matches, item)
			}
		}
		nextPage := stringValue(response.OpcNextPage)
		if nextPage == "" {
			return matches, nil
		}
		if _, ok := seenPages[nextPage]; ok {
			return nil, fmt.Errorf("JavaMigrationAnalysis list pagination repeated page token %q", nextPage)
		}
		seenPages[nextPage] = struct{}{}
		request.Page = response.OpcNextPage
	}
}

func (c *javaMigrationAnalysisRuntimeClient) validateDesiredSpec(resource *jmsutilsv1beta1.JavaMigrationAnalysis) error {
	var missing []string
	if strings.TrimSpace(resource.Spec.CompartmentId) == "" {
		missing = append(missing, "spec.compartmentId")
	}
	if strings.TrimSpace(resource.Spec.AnalysisProjectName) == "" {
		missing = append(missing, "spec.analysisProjectName")
	}
	if len(normalizeJavaMigrationAnalysisStringSlice(resource.Spec.InputApplicationsObjectStoragePaths)) == 0 {
		missing = append(missing, "spec.inputApplicationsObjectStoragePaths")
	}
	if strings.TrimSpace(resource.Spec.TargetJdkVersion) == "" {
		missing = append(missing, "spec.targetJdkVersion")
	}
	if len(missing) == 0 {
		return nil
	}
	err := fmt.Errorf("JavaMigrationAnalysis create request is missing required fields: %s", strings.Join(missing, ", "))
	c.markFailed(resource, err.Error())
	return err
}

func (c *javaMigrationAnalysisRuntimeClient) rejectCreateOnlyDrift(
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
	current jmsutilssdk.JavaMigrationAnalysis,
) error {
	if !hasJavaMigrationAnalysisDesiredSpec(resource) {
		return nil
	}

	var drift []string
	if got, want := stringValue(current.CompartmentId), strings.TrimSpace(resource.Spec.CompartmentId); got != want {
		drift = append(drift, fmt.Sprintf("spec.compartmentId=%q observed=%q", want, got))
	}
	if got, want := stringValue(current.AnalysisProjectName), strings.TrimSpace(resource.Spec.AnalysisProjectName); got != want {
		drift = append(drift, fmt.Sprintf("spec.analysisProjectName=%q observed=%q", want, got))
	}
	if got, want := normalizeJavaMigrationAnalysisStringSlice(current.InputApplicationsObjectStoragePaths), normalizeJavaMigrationAnalysisStringSlice(resource.Spec.InputApplicationsObjectStoragePaths); !slices.Equal(got, want) {
		drift = append(drift, fmt.Sprintf("spec.inputApplicationsObjectStoragePaths=%v observed=%v", want, got))
	}
	if got, want := stringValue(current.TargetJdkVersion), strings.TrimSpace(resource.Spec.TargetJdkVersion); got != want {
		drift = append(drift, fmt.Sprintf("spec.targetJdkVersion=%q observed=%q", want, got))
	}
	if len(drift) == 0 {
		return nil
	}

	err := fmt.Errorf("JavaMigrationAnalysis has create-only drift and this SDK exposes no update operation: %s", strings.Join(drift, "; "))
	c.markFailed(resource, err.Error())
	return err
}

func (c *javaMigrationAnalysisRuntimeClient) projectStatus(
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
	current jmsutilssdk.JavaMigrationAnalysis,
) {
	resource.Status.Id = stringValue(current.Id)
	resource.Status.WorkRequestId = stringValue(current.WorkRequestId)
	resource.Status.CompartmentId = stringValue(current.CompartmentId)
	resource.Status.AnalysisProjectName = stringValue(current.AnalysisProjectName)
	resource.Status.InputApplicationsObjectStoragePaths = append([]string(nil), current.InputApplicationsObjectStoragePaths...)
	resource.Status.TargetJdkVersion = stringValue(current.TargetJdkVersion)
	resource.Status.NamespaceName = stringValue(current.NamespaceName)
	resource.Status.BucketName = stringValue(current.BucketName)
	resource.Status.AnalysisResultObjectStoragePath = stringValue(current.AnalysisResultObjectStoragePath)
	resource.Status.AnalysisResultFiles = append([]string(nil), current.AnalysisResultFiles...)
	resource.Status.TimeCreated = sdkTimeString(current.TimeCreated)
	resource.Status.TimeStarted = sdkTimeString(current.TimeStarted)
	resource.Status.TimeFinished = sdkTimeString(current.TimeFinished)
	resource.Status.CreatedBy = jmsutilsv1beta1.JavaMigrationAnalysisCreatedBy{
		Id:          "",
		DisplayName: "",
	}
	if current.CreatedBy != nil {
		resource.Status.CreatedBy.Id = stringValue(current.CreatedBy.Id)
		resource.Status.CreatedBy.DisplayName = stringValue(current.CreatedBy.DisplayName)
	}
	resource.Status.Metadata = stringValue(current.Metadata)
	if resource.Status.Id != "" {
		resource.Status.OsokStatus.Ocid = shared.OCID(resource.Status.Id)
	}
	if current.TimeCreated != nil {
		createdAt := metav1.NewTime(current.TimeCreated.Time)
		resource.Status.OsokStatus.CreatedAt = &createdAt
	}
}

func (c *javaMigrationAnalysisRuntimeClient) projectWorkRequestStatus(
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
	workRequest jmsutilssdk.WorkRequest,
	phase shared.OSOKAsyncPhase,
	class shared.OSOKAsyncNormalizedClass,
) {
	now := metav1.Now()
	message := fmt.Sprintf("JavaMigrationAnalysis work request %s is %s", stringValue(workRequest.Id), workRequest.Status)
	status := &resource.Status.OsokStatus
	status.UpdatedAt = &now
	status.Message = message
	status.Reason = string(shared.Provisioning)
	status.Async.Current = &shared.OSOKAsyncOperation{
		Source:           shared.OSOKAsyncSourceWorkRequest,
		Phase:            phase,
		WorkRequestID:    stringValue(workRequest.Id),
		RawStatus:        string(workRequest.Status),
		RawOperationType: string(workRequest.OperationType),
		NormalizedClass:  class,
		PercentComplete:  workRequest.PercentComplete,
		Message:          message,
		UpdatedAt:        &now,
	}
	*status = util.UpdateOSOKStatusCondition(*status, shared.Provisioning, v1.ConditionTrue, "", message, c.log)
}

func (c *javaMigrationAnalysisRuntimeClient) markActive(resource *jmsutilsv1beta1.JavaMigrationAnalysis, message string) {
	now := metav1.Now()
	status := &resource.Status.OsokStatus
	status.UpdatedAt = &now
	status.Async.Current = nil
	status.Message = message
	status.Reason = string(shared.Active)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Active, v1.ConditionTrue, "", message, c.log)
}

func (c *javaMigrationAnalysisRuntimeClient) markFailed(resource *jmsutilsv1beta1.JavaMigrationAnalysis, message string) {
	now := metav1.Now()
	status := &resource.Status.OsokStatus
	status.UpdatedAt = &now
	status.Message = message
	status.Reason = string(shared.Failed)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Failed, v1.ConditionFalse, "", message, c.log)
}

func (c *javaMigrationAnalysisRuntimeClient) markTerminating(resource *jmsutilsv1beta1.JavaMigrationAnalysis, message string) {
	now := metav1.Now()
	status := &resource.Status.OsokStatus
	status.UpdatedAt = &now
	status.Message = message
	status.Reason = string(shared.Terminating)
	status.Async.Current = &shared.OSOKAsyncOperation{
		Source:          shared.OSOKAsyncSourceLifecycle,
		Phase:           shared.OSOKAsyncPhaseDelete,
		NormalizedClass: shared.OSOKAsyncClassPending,
		Message:         message,
		UpdatedAt:       &now,
	}
	*status = util.UpdateOSOKStatusCondition(*status, shared.Terminating, v1.ConditionTrue, "", message, c.log)
}

func (c *javaMigrationAnalysisRuntimeClient) markDeleted(resource *jmsutilsv1beta1.JavaMigrationAnalysis, message string) {
	now := metav1.Now()
	status := &resource.Status.OsokStatus
	status.DeletedAt = &now
	status.UpdatedAt = &now
	status.Async.Current = nil
	status.Message = message
	status.Reason = string(shared.Terminating)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Terminating, v1.ConditionTrue, "", message, c.log)
}

func (c *javaMigrationAnalysisRuntimeClient) rejectAmbiguousDelete(
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
	operation string,
	err error,
) error {
	message := fmt.Sprintf("JavaMigrationAnalysis %s returned ambiguous 404 NotAuthorizedOrNotFound; keeping the finalizer until deletion is unambiguously confirmed", operation)
	c.markTerminating(resource, message)
	return fmt.Errorf("%s: %w", message, err)
}

func currentJavaMigrationAnalysisID(resource *jmsutilsv1beta1.JavaMigrationAnalysis) string {
	if id := recordedJavaMigrationAnalysisID(resource); id != "" {
		return id
	}
	return javaMigrationAnalysisAnnotationID(resource)
}

func recordedJavaMigrationAnalysisID(resource *jmsutilsv1beta1.JavaMigrationAnalysis) string {
	if resource == nil {
		return ""
	}
	if id := strings.TrimSpace(string(resource.Status.OsokStatus.Ocid)); id != "" {
		return id
	}
	return strings.TrimSpace(resource.Status.Id)
}

func hasJavaMigrationAnalysisDesiredSpec(resource *jmsutilsv1beta1.JavaMigrationAnalysis) bool {
	if resource == nil {
		return false
	}
	return strings.TrimSpace(resource.Spec.CompartmentId) != "" ||
		strings.TrimSpace(resource.Spec.AnalysisProjectName) != "" ||
		len(normalizeJavaMigrationAnalysisStringSlice(resource.Spec.InputApplicationsObjectStoragePaths)) > 0 ||
		strings.TrimSpace(resource.Spec.TargetJdkVersion) != ""
}

func javaMigrationAnalysisAnnotationID(resource *jmsutilsv1beta1.JavaMigrationAnalysis) string {
	if resource == nil {
		return ""
	}
	return strings.TrimSpace(resource.GetAnnotations()[javaMigrationAnalysisIDAnnotation])
}

func currentJavaMigrationAnalysisWorkRequestID(resource *jmsutilsv1beta1.JavaMigrationAnalysis) string {
	if resource == nil {
		return ""
	}
	if id := strings.TrimSpace(resource.Status.WorkRequestId); id != "" {
		return id
	}
	if resource.Status.OsokStatus.Async.Current != nil {
		return strings.TrimSpace(resource.Status.OsokStatus.Async.Current.WorkRequestID)
	}
	return ""
}

func canBindJavaMigrationAnalysisFromStatus(resource *jmsutilsv1beta1.JavaMigrationAnalysis) bool {
	if resource == nil {
		return false
	}
	return strings.TrimSpace(resource.Status.CompartmentId) != "" &&
		strings.TrimSpace(resource.Status.AnalysisProjectName) != ""
}

func javaMigrationAnalysisDeletePending(resource *jmsutilsv1beta1.JavaMigrationAnalysis) bool {
	if resource == nil || resource.Status.OsokStatus.Async.Current == nil {
		return false
	}
	current := resource.Status.OsokStatus.Async.Current
	return current.Phase == shared.OSOKAsyncPhaseDelete &&
		current.NormalizedClass == shared.OSOKAsyncClassPending
}

func javaMigrationAnalysisSummaryMatchesStatus(
	summary jmsutilssdk.JavaMigrationAnalysisSummary,
	compartmentID string,
	projectName string,
) bool {
	return stringValue(summary.CompartmentId) == compartmentID &&
		stringValue(summary.AnalysisProjectName) == projectName
}

func javaMigrationAnalysisSummaryMatchesSpec(
	summary jmsutilssdk.JavaMigrationAnalysisSummary,
	resource *jmsutilsv1beta1.JavaMigrationAnalysis,
) bool {
	return stringValue(summary.CompartmentId) == strings.TrimSpace(resource.Spec.CompartmentId) &&
		stringValue(summary.AnalysisProjectName) == strings.TrimSpace(resource.Spec.AnalysisProjectName) &&
		slices.Equal(
			normalizeJavaMigrationAnalysisStringSlice(summary.InputApplicationsObjectStoragePaths),
			normalizeJavaMigrationAnalysisStringSlice(resource.Spec.InputApplicationsObjectStoragePaths),
		) &&
		stringValue(summary.TargetJdkVersion) == strings.TrimSpace(resource.Spec.TargetJdkVersion)
}

func javaMigrationAnalysisCreateDetails(resource *jmsutilsv1beta1.JavaMigrationAnalysis) jmsutilssdk.RequestJavaMigrationAnalysisDetails {
	compartmentID := strings.TrimSpace(resource.Spec.CompartmentId)
	projectName := strings.TrimSpace(resource.Spec.AnalysisProjectName)
	targetJDKVersion := strings.TrimSpace(resource.Spec.TargetJdkVersion)
	return jmsutilssdk.RequestJavaMigrationAnalysisDetails{
		CompartmentId: &compartmentID,
		Targets: []jmsutilssdk.JavaMigrationAnalysisTarget{
			{
				AnalysisProjectName:                 &projectName,
				InputApplicationsObjectStoragePaths: normalizeJavaMigrationAnalysisStringSlice(resource.Spec.InputApplicationsObjectStoragePaths),
				TargetJdkVersions:                   []string{targetJDKVersion},
			},
		},
	}
}

func javaMigrationAnalysisRetryToken(resource *jmsutilsv1beta1.JavaMigrationAnalysis) string {
	key := string(resource.UID)
	if strings.TrimSpace(key) == "" {
		key = resource.Namespace + "/" + resource.Name
	}
	if strings.TrimSpace(key) == "/" {
		key = "anonymous-javamigrationanalysis"
	}
	sum := sha256.Sum256([]byte(key))
	return "osok-jma-" + hex.EncodeToString(sum[:16])
}

func normalizeJavaMigrationAnalysisStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func javaMigrationAnalysisWorkRequestClass(status jmsutilssdk.OperationStatusEnum) shared.OSOKAsyncNormalizedClass {
	switch status {
	case jmsutilssdk.OperationStatusAccepted, jmsutilssdk.OperationStatusInProgress, jmsutilssdk.OperationStatusWaiting, jmsutilssdk.OperationStatusCancelling:
		return shared.OSOKAsyncClassPending
	case jmsutilssdk.OperationStatusSucceeded:
		return shared.OSOKAsyncClassSucceeded
	case jmsutilssdk.OperationStatusFailed:
		return shared.OSOKAsyncClassFailed
	case jmsutilssdk.OperationStatusCancelled:
		return shared.OSOKAsyncClassCanceled
	case jmsutilssdk.OperationStatusNeedsAttention:
		return shared.OSOKAsyncClassAttention
	default:
		return shared.OSOKAsyncClassUnknown
	}
}

func javaMigrationAnalysisIDFromWorkRequest(workRequest jmsutilssdk.WorkRequest) string {
	for _, resource := range workRequest.Resources {
		identifier := stringValue(resource.Identifier)
		if identifier == "" {
			continue
		}
		entityType := strings.ToLower(strings.ReplaceAll(stringValue(resource.EntityType), " ", ""))
		if strings.Contains(entityType, "javamigrationanalysis") {
			return identifier
		}
	}
	for _, resource := range workRequest.Resources {
		identifier := stringValue(resource.Identifier)
		if identifier == "" {
			continue
		}
		switch resource.ActionType {
		case jmsutilssdk.ActionTypeCreated, jmsutilssdk.ActionTypeInProgress, jmsutilssdk.ActionTypeRelated:
			return identifier
		}
	}
	return ""
}

func javaMigrationAnalysisIsUnambiguousNotFound(err error) bool {
	return errorutil.ClassifyDeleteError(err).IsUnambiguousNotFound()
}

func javaMigrationAnalysisIsAuthShapedNotFound(err error) bool {
	return errorutil.ClassifyDeleteError(err).IsAuthShapedNotFound()
}

func sdkTimeString(value *common.SDKTime) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
