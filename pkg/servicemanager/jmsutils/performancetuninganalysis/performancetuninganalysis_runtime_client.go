/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package performancetuninganalysis

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	jmsutilssdk "github.com/oracle/oci-go-sdk/v65/jmsutils"
	jmsutilsv1beta1 "github.com/oracle/oci-service-operator/api/jmsutils/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	"github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	performanceTuningAnalysisKind = "PerformanceTuningAnalysis"

	PerformanceTuningAnalysisIDAnnotation                        = "jmsutils.oracle.com/performance-tuning-analysis-id"
	PerformanceTuningAnalysisCompartmentIDAnnotation             = "jmsutils.oracle.com/compartment-id"
	PerformanceTuningAnalysisAnalysisProjectNameAnnotation       = "jmsutils.oracle.com/analysis-project-name"
	PerformanceTuningAnalysisArtifactObjectStoragePathAnnotation = "jmsutils.oracle.com/artifact-object-storage-path"

	performanceTuningAnalysisDefaultRequeue = time.Minute
)

type performanceTuningAnalysisOCIClient interface {
	RequestPerformanceTuningAnalysis(context.Context, jmsutilssdk.RequestPerformanceTuningAnalysisRequest) (jmsutilssdk.RequestPerformanceTuningAnalysisResponse, error)
	GetPerformanceTuningAnalysis(context.Context, jmsutilssdk.GetPerformanceTuningAnalysisRequest) (jmsutilssdk.GetPerformanceTuningAnalysisResponse, error)
	ListPerformanceTuningAnalysis(context.Context, jmsutilssdk.ListPerformanceTuningAnalysisRequest) (jmsutilssdk.ListPerformanceTuningAnalysisResponse, error)
	DeletePerformanceTuningAnalysis(context.Context, jmsutilssdk.DeletePerformanceTuningAnalysisRequest) (jmsutilssdk.DeletePerformanceTuningAnalysisResponse, error)
	GetWorkRequest(context.Context, jmsutilssdk.GetWorkRequestRequest) (jmsutilssdk.GetWorkRequestResponse, error)
}

type performanceTuningAnalysisRuntimeClient struct {
	client  performanceTuningAnalysisOCIClient
	initErr error
	log     loggerutil.OSOKLogger
}

type performanceTuningAnalysisIdentity struct {
	id                        string
	compartmentID             string
	analysisProjectName       string
	artifactObjectStoragePath string
	workRequestID             string
}

func init() {
	registerPerformanceTuningAnalysisRuntimeHooksMutator(func(manager *PerformanceTuningAnalysisServiceManager, hooks *PerformanceTuningAnalysisRuntimeHooks) {
		client, initErr := newPerformanceTuningAnalysisSDKClient(manager)
		applyPerformanceTuningAnalysisRuntimeHooks(manager, hooks, client, initErr)
	})
}

func newPerformanceTuningAnalysisSDKClient(manager *PerformanceTuningAnalysisServiceManager) (performanceTuningAnalysisOCIClient, error) {
	if manager == nil {
		return nil, fmt.Errorf("%s service manager is nil", performanceTuningAnalysisKind)
	}
	client, err := jmsutilssdk.NewJmsUtilsClientWithConfigurationProvider(manager.Provider)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func applyPerformanceTuningAnalysisRuntimeHooks(
	manager *PerformanceTuningAnalysisServiceManager,
	hooks *PerformanceTuningAnalysisRuntimeHooks,
	client performanceTuningAnalysisOCIClient,
	initErr error,
) {
	if hooks == nil {
		return
	}
	hooks.Semantics = performanceTuningAnalysisRuntimeSemantics()
	hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(PerformanceTuningAnalysisServiceClient) PerformanceTuningAnalysisServiceClient {
		return newPerformanceTuningAnalysisRuntimeClient(manager, client, initErr)
	})
}

func newPerformanceTuningAnalysisRuntimeClient(
	manager *PerformanceTuningAnalysisServiceManager,
	client performanceTuningAnalysisOCIClient,
	initErr error,
) PerformanceTuningAnalysisServiceClient {
	runtimeClient := &performanceTuningAnalysisRuntimeClient{
		client:  client,
		initErr: initErr,
	}
	if manager != nil {
		runtimeClient.log = manager.Log
	}
	return runtimeClient
}

func newPerformanceTuningAnalysisServiceClientWithOCIClient(
	client performanceTuningAnalysisOCIClient,
	log loggerutil.OSOKLogger,
) PerformanceTuningAnalysisServiceClient {
	return &performanceTuningAnalysisRuntimeClient{
		client: client,
		log:    log,
	}
}

func performanceTuningAnalysisRuntimeSemantics() *generatedruntime.Semantics {
	return &generatedruntime.Semantics{
		FormalService: "jmsutils",
		FormalSlug:    "performancetuninganalysis",
		Async: &generatedruntime.AsyncSemantics{
			Strategy:             "workrequest",
			Runtime:              "handwritten",
			FormalClassification: "workrequest",
		},
		StatusProjection:  "required",
		SecretSideEffects: "none",
		FinalizerPolicy:   "retain-until-confirmed-delete",
		List: &generatedruntime.ListSemantics{
			ResponseItemsField: "Items",
			MatchFields:        []string{"compartmentId", "analysisProjectName", "artifactObjectStoragePath", "workRequestId"},
		},
		Mutation: generatedruntime.MutationSemantics{
			ForceNew: []string{"compartmentId", "analysisProjectName", "artifactObjectStoragePath"},
		},
		Hooks: generatedruntime.HookSet{
			Create: []generatedruntime.Hook{{Helper: "resource-local PerformanceTuningAnalysis runtime", EntityType: performanceTuningAnalysisKind, Action: "RequestPerformanceTuningAnalysis"}},
			Delete: []generatedruntime.Hook{{Helper: "resource-local PerformanceTuningAnalysis runtime", EntityType: performanceTuningAnalysisKind, Action: "DeletePerformanceTuningAnalysis"}},
		},
		CreateFollowUp: generatedruntime.FollowUpSemantics{Strategy: "workrequest-then-read"},
		DeleteFollowUp: generatedruntime.FollowUpSemantics{Strategy: "confirm-delete"},
		Unsupported: []generatedruntime.UnsupportedSemantic{{
			Category: "crd-shape",
			StopCondition: fmt.Sprintf(
				"%s, %s, and %s annotations are required until RequestPerformanceTuningAnalysis inputs are promoted into the CR spec",
				PerformanceTuningAnalysisCompartmentIDAnnotation,
				PerformanceTuningAnalysisAnalysisProjectNameAnnotation,
				PerformanceTuningAnalysisArtifactObjectStoragePathAnnotation,
			),
		}},
	}
}

func (c *performanceTuningAnalysisRuntimeClient) CreateOrUpdate(
	ctx context.Context,
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	_ ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	if err := c.validateConfigured(resource); err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
	}

	identity, err := resolvePerformanceTuningAnalysisIdentity(resource)
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
	}

	if identity.id != "" {
		return c.observePerformanceTuningAnalysis(ctx, resource, identity.id)
	}

	if identity.workRequestID != "" {
		return c.resumePerformanceTuningAnalysisCreate(ctx, resource, identity)
	}

	existing, err := c.findPerformanceTuningAnalysis(ctx, identity, "")
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
	}
	if existing != nil {
		return c.observePerformanceTuningAnalysis(ctx, resource, stringValue(existing.Id))
	}

	return c.requestPerformanceTuningAnalysis(ctx, resource, identity)
}

func (c *performanceTuningAnalysisRuntimeClient) Delete(
	ctx context.Context,
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
) (bool, error) {
	if err := c.validateConfigured(resource); err != nil {
		return false, c.fail(resource, err)
	}

	identity := resolvePerformanceTuningAnalysisDeleteIdentity(resource)
	if identity.id == "" {
		if identity.workRequestID != "" {
			return c.deleteAfterCreateWorkRequest(ctx, resource, identity)
		}
		c.markDeleted(resource, "OCI PerformanceTuningAnalysis was not created")
		return true, nil
	}

	if deleted, err, handled := c.confirmPerformanceTuningAnalysisAlreadyDeleted(ctx, resource, identity); handled {
		return deleted, err
	}

	response, err := c.client.DeletePerformanceTuningAnalysis(ctx, jmsutilssdk.DeletePerformanceTuningAnalysisRequest{
		PerformanceTuningAnalysisId: common.String(identity.id),
	})
	if err != nil {
		return c.handleDeleteError(ctx, resource, identity, err)
	}
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)

	return c.confirmPerformanceTuningAnalysisDeleted(ctx, resource, identity, "after delete request")
}

func (c *performanceTuningAnalysisRuntimeClient) validateConfigured(resource *jmsutilsv1beta1.PerformanceTuningAnalysis) error {
	if resource == nil {
		return fmt.Errorf("%s resource is nil", performanceTuningAnalysisKind)
	}
	if c.initErr != nil {
		return fmt.Errorf("initialize %s OCI client: %w", performanceTuningAnalysisKind, c.initErr)
	}
	if c.client == nil {
		return fmt.Errorf("%s OCI client is not configured", performanceTuningAnalysisKind)
	}
	return nil
}

func resolvePerformanceTuningAnalysisIdentity(resource *jmsutilsv1beta1.PerformanceTuningAnalysis) (performanceTuningAnalysisIdentity, error) {
	if resource == nil {
		return performanceTuningAnalysisIdentity{}, fmt.Errorf("%s resource is nil", performanceTuningAnalysisKind)
	}

	annotations := resource.GetAnnotations()
	statusID := firstNonEmptyString(resource.Status.Id, string(resource.Status.OsokStatus.Ocid))
	annotationID := annotationValue(annotations, PerformanceTuningAnalysisIDAnnotation)
	if hasConflictingPerformanceTuningAnalysisID(statusID, annotationID) {
		return performanceTuningAnalysisIdentity{}, fmt.Errorf("%s create-only identity annotation %q changed; create a replacement resource instead", performanceTuningAnalysisKind, PerformanceTuningAnalysisIDAnnotation)
	}

	identity := performanceTuningAnalysisIdentity{
		id:                        firstNonEmptyString(statusID, annotationID),
		compartmentID:             firstNonEmptyString(resource.Status.CompartmentId, annotationValue(annotations, PerformanceTuningAnalysisCompartmentIDAnnotation)),
		analysisProjectName:       firstNonEmptyString(resource.Status.AnalysisProjectName, annotationValue(annotations, PerformanceTuningAnalysisAnalysisProjectNameAnnotation)),
		artifactObjectStoragePath: firstNonEmptyString(resource.Status.ArtifactObjectStoragePath, annotationValue(annotations, PerformanceTuningAnalysisArtifactObjectStoragePathAnnotation)),
		workRequestID:             currentPerformanceTuningAnalysisWorkRequestID(resource),
	}

	if err := rejectPerformanceTuningAnalysisAnnotationDrift(resource, annotations); err != nil {
		return performanceTuningAnalysisIdentity{}, err
	}
	if identity.id != "" || identity.workRequestID != "" {
		return identity, nil
	}
	if err := requirePerformanceTuningAnalysisCreateIdentity(identity); err != nil {
		return performanceTuningAnalysisIdentity{}, err
	}
	return identity, nil
}

func hasConflictingPerformanceTuningAnalysisID(statusID string, annotationID string) bool {
	statusID = strings.TrimSpace(statusID)
	annotationID = strings.TrimSpace(annotationID)
	return statusID != "" && annotationID != "" && statusID != annotationID
}

func requirePerformanceTuningAnalysisCreateIdentity(identity performanceTuningAnalysisIdentity) error {
	for _, required := range []struct {
		value      string
		annotation string
		field      string
	}{
		{value: identity.compartmentID, annotation: PerformanceTuningAnalysisCompartmentIDAnnotation, field: "compartmentId"},
		{value: identity.analysisProjectName, annotation: PerformanceTuningAnalysisAnalysisProjectNameAnnotation, field: "analysisProjectName"},
		{value: identity.artifactObjectStoragePath, annotation: PerformanceTuningAnalysisArtifactObjectStoragePathAnnotation, field: "artifactObjectStoragePath"},
	} {
		if strings.TrimSpace(required.value) == "" {
			return fmt.Errorf("%s requires metadata annotation %q because the CRD has no spec %s field", performanceTuningAnalysisKind, required.annotation, required.field)
		}
	}
	return nil
}

func resolvePerformanceTuningAnalysisDeleteIdentity(resource *jmsutilsv1beta1.PerformanceTuningAnalysis) performanceTuningAnalysisIdentity {
	if resource == nil {
		return performanceTuningAnalysisIdentity{}
	}
	annotations := resource.GetAnnotations()
	return performanceTuningAnalysisIdentity{
		id:                        firstNonEmptyString(resource.Status.Id, string(resource.Status.OsokStatus.Ocid), annotationValue(annotations, PerformanceTuningAnalysisIDAnnotation)),
		compartmentID:             firstNonEmptyString(resource.Status.CompartmentId, annotationValue(annotations, PerformanceTuningAnalysisCompartmentIDAnnotation)),
		analysisProjectName:       firstNonEmptyString(resource.Status.AnalysisProjectName, annotationValue(annotations, PerformanceTuningAnalysisAnalysisProjectNameAnnotation)),
		artifactObjectStoragePath: firstNonEmptyString(resource.Status.ArtifactObjectStoragePath, annotationValue(annotations, PerformanceTuningAnalysisArtifactObjectStoragePathAnnotation)),
		workRequestID:             currentPerformanceTuningAnalysisWorkRequestID(resource),
	}
}

func rejectPerformanceTuningAnalysisAnnotationDrift(resource *jmsutilsv1beta1.PerformanceTuningAnalysis, annotations map[string]string) error {
	for _, field := range []struct {
		statusValue string
		annotation  string
		label       string
	}{
		{statusValue: resource.Status.CompartmentId, annotation: PerformanceTuningAnalysisCompartmentIDAnnotation, label: "compartmentId"},
		{statusValue: resource.Status.AnalysisProjectName, annotation: PerformanceTuningAnalysisAnalysisProjectNameAnnotation, label: "analysisProjectName"},
		{statusValue: resource.Status.ArtifactObjectStoragePath, annotation: PerformanceTuningAnalysisArtifactObjectStoragePathAnnotation, label: "artifactObjectStoragePath"},
	} {
		annotatedValue := annotationValue(annotations, field.annotation)
		if strings.TrimSpace(field.statusValue) != "" && annotatedValue != "" && strings.TrimSpace(field.statusValue) != annotatedValue {
			return fmt.Errorf("%s create-only %s annotation %q changed; create a replacement resource instead", performanceTuningAnalysisKind, field.label, field.annotation)
		}
	}
	return nil
}

func (c *performanceTuningAnalysisRuntimeClient) observePerformanceTuningAnalysis(
	ctx context.Context,
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	id string,
) (servicemanager.OSOKResponse, error) {
	response, err := c.client.GetPerformanceTuningAnalysis(ctx, jmsutilssdk.GetPerformanceTuningAnalysisRequest{
		PerformanceTuningAnalysisId: common.String(id),
	})
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
	}

	c.projectPerformanceTuningAnalysis(resource, response.PerformanceTuningAnalysis, servicemanager.ResponseOpcRequestID(response))
	return servicemanager.OSOKResponse{IsSuccessful: true}, nil
}

func (c *performanceTuningAnalysisRuntimeClient) requestPerformanceTuningAnalysis(
	ctx context.Context,
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	identity performanceTuningAnalysisIdentity,
) (servicemanager.OSOKResponse, error) {
	response, err := c.client.RequestPerformanceTuningAnalysis(ctx, jmsutilssdk.RequestPerformanceTuningAnalysisRequest{
		RequestPerformanceTuningAnalysisDetails: jmsutilssdk.RequestPerformanceTuningAnalysisDetails{
			CompartmentId: common.String(identity.compartmentID),
			Targets: []jmsutilssdk.PerformanceTuningAnalysisTarget{{
				AnalysisProjectName:       common.String(identity.analysisProjectName),
				ArtifactObjectStoragePath: common.String(identity.artifactObjectStoragePath),
			}},
		},
		OpcRetryToken: common.String(performanceTuningAnalysisRetryToken(resource, identity)),
	})
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
	}
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)

	workRequestID := stringValue(response.OpcWorkRequestId)
	if workRequestID == "" {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, fmt.Errorf("%s create did not return an opc-work-request-id", performanceTuningAnalysisKind))
	}

	resource.Status.WorkRequestId = workRequestID
	resource.Status.CompartmentId = identity.compartmentID
	resource.Status.AnalysisProjectName = identity.analysisProjectName
	resource.Status.ArtifactObjectStoragePath = identity.artifactObjectStoragePath
	return c.markWorkRequest(resource, workRequestID, string(jmsutilssdk.OperationStatusAccepted), string(jmsutilssdk.OperationTypeRequestPerformanceTuningSaAnalysis), nil, shared.OSOKAsyncPhaseCreate, shared.OSOKAsyncClassPending, fmt.Sprintf("%s create work request %s is ACCEPTED", performanceTuningAnalysisKind, workRequestID)), nil
}

func (c *performanceTuningAnalysisRuntimeClient) resumePerformanceTuningAnalysisCreate(
	ctx context.Context,
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	identity performanceTuningAnalysisIdentity,
) (servicemanager.OSOKResponse, error) {
	response, err := c.client.GetWorkRequest(ctx, jmsutilssdk.GetWorkRequestRequest{
		WorkRequestId: common.String(identity.workRequestID),
	})
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
	}
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)

	current, err := performanceTuningAnalysisWorkRequestAsync(response.WorkRequest, shared.OSOKAsyncPhaseCreate)
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
	}

	switch current.NormalizedClass {
	case shared.OSOKAsyncClassPending:
		return c.applyWorkRequest(resource, current), nil
	case shared.OSOKAsyncClassFailed, shared.OSOKAsyncClassCanceled, shared.OSOKAsyncClassAttention, shared.OSOKAsyncClassUnknown:
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, fmt.Errorf("%s create work request %s finished with status %s", performanceTuningAnalysisKind, identity.workRequestID, current.RawStatus))
	case shared.OSOKAsyncClassSucceeded:
		return c.completePerformanceTuningAnalysisCreate(ctx, resource, identity, response.WorkRequest, current)
	default:
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, fmt.Errorf("%s create work request %s projected unsupported async class %s", performanceTuningAnalysisKind, identity.workRequestID, current.NormalizedClass))
	}
}

func (c *performanceTuningAnalysisRuntimeClient) completePerformanceTuningAnalysisCreate(
	ctx context.Context,
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	identity performanceTuningAnalysisIdentity,
	workRequest jmsutilssdk.WorkRequest,
	current *shared.OSOKAsyncOperation,
) (servicemanager.OSOKResponse, error) {
	resourceID := performanceTuningAnalysisIDFromWorkRequest(workRequest)
	if resourceID != "" {
		response, err := c.client.GetPerformanceTuningAnalysis(ctx, jmsutilssdk.GetPerformanceTuningAnalysisRequest{
			PerformanceTuningAnalysisId: common.String(resourceID),
		})
		if err == nil {
			c.projectPerformanceTuningAnalysis(resource, response.PerformanceTuningAnalysis, servicemanager.ResponseOpcRequestID(response))
			return servicemanager.OSOKResponse{IsSuccessful: true}, nil
		}
		if !isPerformanceTuningAnalysisUnambiguousNotFound(err) {
			return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
		}
	}

	existing, err := c.findPerformanceTuningAnalysis(ctx, identity, identity.workRequestID)
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
	}
	if existing != nil {
		c.projectPerformanceTuningAnalysisSummary(resource, *existing, "")
		return servicemanager.OSOKResponse{IsSuccessful: true}, nil
	}

	waiting := *current
	waiting.NormalizedClass = shared.OSOKAsyncClassPending
	waiting.Message = fmt.Sprintf("%s create work request %s succeeded; waiting for PerformanceTuningAnalysis to become readable", performanceTuningAnalysisKind, identity.workRequestID)
	waiting.UpdatedAt = nil
	return c.applyWorkRequest(resource, &waiting), nil
}

func (c *performanceTuningAnalysisRuntimeClient) deleteAfterCreateWorkRequest(
	ctx context.Context,
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	identity performanceTuningAnalysisIdentity,
) (bool, error) {
	response, err := c.client.GetWorkRequest(ctx, jmsutilssdk.GetWorkRequestRequest{
		WorkRequestId: common.String(identity.workRequestID),
	})
	if err != nil {
		return false, c.fail(resource, err)
	}
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)

	current, err := performanceTuningAnalysisWorkRequestAsync(response.WorkRequest, shared.OSOKAsyncPhaseCreate)
	if err != nil {
		return false, c.fail(resource, err)
	}

	switch current.NormalizedClass {
	case shared.OSOKAsyncClassPending:
		c.markTerminating(resource, fmt.Sprintf("%s delete is waiting for create work request %s to finish", performanceTuningAnalysisKind, identity.workRequestID))
		return false, nil
	case shared.OSOKAsyncClassFailed, shared.OSOKAsyncClassCanceled, shared.OSOKAsyncClassAttention, shared.OSOKAsyncClassUnknown:
		c.markDeleted(resource, fmt.Sprintf("%s create work request %s finished with status %s before delete", performanceTuningAnalysisKind, identity.workRequestID, current.RawStatus))
		return true, nil
	case shared.OSOKAsyncClassSucceeded:
		resourceID := performanceTuningAnalysisIDFromWorkRequest(response.WorkRequest)
		if resourceID == "" {
			existing, err := c.findPerformanceTuningAnalysis(ctx, identity, identity.workRequestID)
			if err != nil {
				return false, c.fail(resource, err)
			}
			if existing != nil {
				resourceID = stringValue(existing.Id)
			}
		}
		if resourceID == "" {
			c.markTerminating(resource, fmt.Sprintf("%s create work request %s succeeded; waiting for created resource identity before delete", performanceTuningAnalysisKind, identity.workRequestID))
			return false, nil
		}
		identity.id = resourceID
		resource.Status.Id = resourceID
		resource.Status.OsokStatus.Ocid = shared.OCID(resourceID)
		return c.Delete(ctx, resource)
	default:
		return false, c.fail(resource, fmt.Errorf("%s create work request %s projected unsupported async class %s", performanceTuningAnalysisKind, identity.workRequestID, current.NormalizedClass))
	}
}

func (c *performanceTuningAnalysisRuntimeClient) findPerformanceTuningAnalysis(
	ctx context.Context,
	identity performanceTuningAnalysisIdentity,
	requiredWorkRequestID string,
) (*jmsutilssdk.PerformanceTuningAnalysisSummary, error) {
	if identity.compartmentID == "" || identity.analysisProjectName == "" {
		return nil, nil
	}

	var matches []jmsutilssdk.PerformanceTuningAnalysisSummary
	page := ""
	for {
		response, err := c.listPerformanceTuningAnalysisPage(ctx, identity, page)
		if err != nil {
			return nil, err
		}
		matches = appendPerformanceTuningAnalysisMatches(matches, response.Items, identity, requiredWorkRequestID)
		page = stringValue(response.OpcNextPage)
		if page == "" {
			break
		}
	}

	return singlePerformanceTuningAnalysisMatch(matches, identity)
}

func (c *performanceTuningAnalysisRuntimeClient) listPerformanceTuningAnalysisPage(
	ctx context.Context,
	identity performanceTuningAnalysisIdentity,
	page string,
) (jmsutilssdk.ListPerformanceTuningAnalysisResponse, error) {
	request := jmsutilssdk.ListPerformanceTuningAnalysisRequest{
		CompartmentId:       common.String(identity.compartmentID),
		AnalysisProjectName: common.String(identity.analysisProjectName),
	}
	if page != "" {
		request.Page = common.String(page)
	}
	return c.client.ListPerformanceTuningAnalysis(ctx, request)
}

func appendPerformanceTuningAnalysisMatches(
	matches []jmsutilssdk.PerformanceTuningAnalysisSummary,
	items []jmsutilssdk.PerformanceTuningAnalysisSummary,
	identity performanceTuningAnalysisIdentity,
	requiredWorkRequestID string,
) []jmsutilssdk.PerformanceTuningAnalysisSummary {
	for _, item := range items {
		if performanceTuningAnalysisSummaryMatches(item, identity, requiredWorkRequestID) {
			matches = append(matches, item)
		}
	}
	return matches
}

func singlePerformanceTuningAnalysisMatch(
	matches []jmsutilssdk.PerformanceTuningAnalysisSummary,
	identity performanceTuningAnalysisIdentity,
) (*jmsutilssdk.PerformanceTuningAnalysisSummary, error) {
	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("%s lookup matched %d analyses for compartment %q, project %q, and artifact %q; use %q to bind a specific resource", performanceTuningAnalysisKind, len(matches), identity.compartmentID, identity.analysisProjectName, identity.artifactObjectStoragePath, PerformanceTuningAnalysisIDAnnotation)
	}
	return &matches[0], nil
}

func performanceTuningAnalysisSummaryMatches(
	item jmsutilssdk.PerformanceTuningAnalysisSummary,
	identity performanceTuningAnalysisIdentity,
	requiredWorkRequestID string,
) bool {
	if requiredWorkRequestID != "" && stringValue(item.WorkRequestId) != requiredWorkRequestID {
		return false
	}
	if identity.compartmentID != "" && stringValue(item.CompartmentId) != identity.compartmentID {
		return false
	}
	if identity.analysisProjectName != "" && stringValue(item.AnalysisProjectName) != identity.analysisProjectName {
		return false
	}
	if identity.artifactObjectStoragePath != "" && stringValue(item.ArtifactObjectStoragePath) != identity.artifactObjectStoragePath {
		return false
	}
	return true
}

func (c *performanceTuningAnalysisRuntimeClient) confirmPerformanceTuningAnalysisAlreadyDeleted(
	ctx context.Context,
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	identity performanceTuningAnalysisIdentity,
) (bool, error, bool) {
	_, err := c.client.GetPerformanceTuningAnalysis(ctx, jmsutilssdk.GetPerformanceTuningAnalysisRequest{
		PerformanceTuningAnalysisId: common.String(identity.id),
	})
	if err == nil {
		return false, nil, false
	}
	if isPerformanceTuningAnalysisAmbiguousNotFound(err) {
		return false, c.fail(resource, fmt.Errorf("%s delete confirmation returned ambiguous 404 NotAuthorizedOrNotFound; refusing to confirm deletion: %w", performanceTuningAnalysisKind, err)), true
	}
	if !isPerformanceTuningAnalysisUnambiguousNotFound(err) {
		return false, c.fail(resource, err), true
	}

	deleted, err := c.confirmPerformanceTuningAnalysisAbsentFromList(ctx, resource, identity, "already absent")
	return deleted, err, true
}

func (c *performanceTuningAnalysisRuntimeClient) handleDeleteError(
	ctx context.Context,
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	identity performanceTuningAnalysisIdentity,
	err error,
) (bool, error) {
	if isPerformanceTuningAnalysisAmbiguousNotFound(err) {
		return false, c.fail(resource, fmt.Errorf("%s delete returned ambiguous 404 NotAuthorizedOrNotFound; keeping the finalizer until deletion is unambiguously confirmed: %w", performanceTuningAnalysisKind, err))
	}
	if isPerformanceTuningAnalysisUnambiguousNotFound(err) {
		return c.confirmPerformanceTuningAnalysisAbsentFromList(ctx, resource, identity, "delete returned not found")
	}
	return false, c.fail(resource, err)
}

func (c *performanceTuningAnalysisRuntimeClient) confirmPerformanceTuningAnalysisDeleted(
	ctx context.Context,
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	identity performanceTuningAnalysisIdentity,
	stage string,
) (bool, error) {
	_, err := c.client.GetPerformanceTuningAnalysis(ctx, jmsutilssdk.GetPerformanceTuningAnalysisRequest{
		PerformanceTuningAnalysisId: common.String(identity.id),
	})
	if err == nil {
		c.markTerminating(resource, fmt.Sprintf("%s delete is in progress", performanceTuningAnalysisKind))
		return false, nil
	}
	if isPerformanceTuningAnalysisAmbiguousNotFound(err) {
		return false, c.fail(resource, fmt.Errorf("%s delete confirmation returned ambiguous 404 NotAuthorizedOrNotFound; refusing to confirm deletion: %w", performanceTuningAnalysisKind, err))
	}
	if !isPerformanceTuningAnalysisUnambiguousNotFound(err) {
		return false, c.fail(resource, err)
	}
	return c.confirmPerformanceTuningAnalysisAbsentFromList(ctx, resource, identity, stage)
}

func (c *performanceTuningAnalysisRuntimeClient) confirmPerformanceTuningAnalysisAbsentFromList(
	ctx context.Context,
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	identity performanceTuningAnalysisIdentity,
	stage string,
) (bool, error) {
	existing, err := c.findPerformanceTuningAnalysis(ctx, identity, "")
	if err != nil {
		return false, c.fail(resource, err)
	}
	if existing != nil && stringValue(existing.Id) == identity.id {
		c.markTerminating(resource, fmt.Sprintf("%s delete is in progress", performanceTuningAnalysisKind))
		return false, nil
	}
	c.markDeleted(resource, fmt.Sprintf("OCI %s deletion confirmed (%s)", performanceTuningAnalysisKind, stage))
	return true, nil
}

func performanceTuningAnalysisWorkRequestAsync(
	workRequest jmsutilssdk.WorkRequest,
	fallbackPhase shared.OSOKAsyncPhase,
) (*shared.OSOKAsyncOperation, error) {
	if workRequest.OperationType != "" && workRequest.OperationType != jmsutilssdk.OperationTypeRequestPerformanceTuningSaAnalysis {
		return nil, fmt.Errorf("%s work request %s exposes unsupported operation type %q", performanceTuningAnalysisKind, stringValue(workRequest.Id), workRequest.OperationType)
	}

	now := metav1.Now()
	current, err := servicemanager.BuildWorkRequestAsyncOperation(&shared.OSOKStatus{}, performanceTuningAnalysisWorkRequestAdapter(), servicemanager.WorkRequestAsyncInput{
		RawStatus:        string(workRequest.Status),
		RawOperationType: string(workRequest.OperationType),
		WorkRequestID:    stringValue(workRequest.Id),
		PercentComplete:  cloneFloat32Ptr(workRequest.PercentComplete),
		FallbackPhase:    fallbackPhase,
	})
	if err != nil {
		return nil, err
	}
	current.UpdatedAt = &now
	if current.Message == "" {
		current.Message = fmt.Sprintf("%s %s work request %s is %s", performanceTuningAnalysisKind, current.Phase, current.WorkRequestID, current.RawStatus)
	}
	return current, nil
}

func performanceTuningAnalysisWorkRequestAdapter() servicemanager.WorkRequestAsyncAdapter {
	return servicemanager.WorkRequestAsyncAdapter{
		PendingStatusTokens: []string{
			string(jmsutilssdk.OperationStatusAccepted),
			string(jmsutilssdk.OperationStatusInProgress),
			string(jmsutilssdk.OperationStatusWaiting),
			string(jmsutilssdk.OperationStatusCancelling),
		},
		SucceededStatusTokens: []string{string(jmsutilssdk.OperationStatusSucceeded)},
		FailedStatusTokens:    []string{string(jmsutilssdk.OperationStatusFailed)},
		CanceledStatusTokens:  []string{string(jmsutilssdk.OperationStatusCancelled)},
		AttentionStatusTokens: []string{string(jmsutilssdk.OperationStatusNeedsAttention)},
		CreateActionTokens:    []string{string(jmsutilssdk.OperationTypeRequestPerformanceTuningSaAnalysis)},
	}
}

func (c *performanceTuningAnalysisRuntimeClient) applyWorkRequest(
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	current *shared.OSOKAsyncOperation,
) servicemanager.OSOKResponse {
	status := &resource.Status.OsokStatus
	if current != nil && current.WorkRequestID != "" {
		resource.Status.WorkRequestId = current.WorkRequestID
	}
	projection := servicemanager.ApplyAsyncOperation(status, current, c.log)
	return servicemanager.OSOKResponse{
		IsSuccessful:    projection.Condition != shared.Failed,
		ShouldRequeue:   projection.ShouldRequeue,
		RequeueDuration: performanceTuningAnalysisDefaultRequeue,
	}
}

func (c *performanceTuningAnalysisRuntimeClient) markWorkRequest(
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	workRequestID string,
	rawStatus string,
	rawOperationType string,
	percentComplete *float32,
	phase shared.OSOKAsyncPhase,
	class shared.OSOKAsyncNormalizedClass,
	message string,
) servicemanager.OSOKResponse {
	now := metav1.Now()
	if resource.Status.OsokStatus.RequestedAt == nil {
		resource.Status.OsokStatus.RequestedAt = &now
	}
	current := &shared.OSOKAsyncOperation{
		Source:           shared.OSOKAsyncSourceWorkRequest,
		Phase:            phase,
		WorkRequestID:    workRequestID,
		RawStatus:        rawStatus,
		RawOperationType: rawOperationType,
		NormalizedClass:  class,
		PercentComplete:  cloneFloat32Ptr(percentComplete),
		Message:          message,
		UpdatedAt:        &now,
	}
	return c.applyWorkRequest(resource, current)
}

func (c *performanceTuningAnalysisRuntimeClient) projectPerformanceTuningAnalysis(
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	analysis jmsutilssdk.PerformanceTuningAnalysis,
	opcRequestID string,
) {
	osokStatus := resource.Status.OsokStatus
	resource.Status = jmsutilsv1beta1.PerformanceTuningAnalysisStatus{OsokStatus: osokStatus}
	resource.Status.Id = stringValue(analysis.Id)
	resource.Status.WorkRequestId = stringValue(analysis.WorkRequestId)
	resource.Status.CompartmentId = stringValue(analysis.CompartmentId)
	resource.Status.AnalysisProjectName = stringValue(analysis.AnalysisProjectName)
	resource.Status.WarningCount = intValue(analysis.WarningCount)
	resource.Status.Result = string(analysis.Result)
	resource.Status.ResultObjectStoragePath = stringValue(analysis.ResultObjectStoragePath)
	resource.Status.ArtifactObjectStoragePath = stringValue(analysis.ArtifactObjectStoragePath)
	resource.Status.TimeCreated = sdkTimeString(analysis.TimeCreated)
	resource.Status.TimeStarted = sdkTimeString(analysis.TimeStarted)
	resource.Status.TimeFinished = sdkTimeString(analysis.TimeFinished)
	resource.Status.CreatedBy = projectPerformanceTuningAnalysisPrincipal(analysis.CreatedBy)
	c.markActive(resource, resource.Status.Id, opcRequestID)
}

func (c *performanceTuningAnalysisRuntimeClient) projectPerformanceTuningAnalysisSummary(
	resource *jmsutilsv1beta1.PerformanceTuningAnalysis,
	summary jmsutilssdk.PerformanceTuningAnalysisSummary,
	opcRequestID string,
) {
	osokStatus := resource.Status.OsokStatus
	resource.Status = jmsutilsv1beta1.PerformanceTuningAnalysisStatus{OsokStatus: osokStatus}
	resource.Status.Id = stringValue(summary.Id)
	resource.Status.WorkRequestId = stringValue(summary.WorkRequestId)
	resource.Status.CompartmentId = stringValue(summary.CompartmentId)
	resource.Status.AnalysisProjectName = stringValue(summary.AnalysisProjectName)
	resource.Status.WarningCount = intValue(summary.WarningCount)
	resource.Status.Result = string(summary.Result)
	resource.Status.ResultObjectStoragePath = stringValue(summary.ResultObjectStoragePath)
	resource.Status.ArtifactObjectStoragePath = stringValue(summary.ArtifactObjectStoragePath)
	resource.Status.TimeCreated = sdkTimeString(summary.TimeCreated)
	resource.Status.TimeStarted = sdkTimeString(summary.TimeStarted)
	resource.Status.TimeFinished = sdkTimeString(summary.TimeFinished)
	resource.Status.CreatedBy = projectPerformanceTuningAnalysisPrincipal(summary.CreatedBy)
	c.markActive(resource, resource.Status.Id, opcRequestID)
}

func (c *performanceTuningAnalysisRuntimeClient) markActive(resource *jmsutilsv1beta1.PerformanceTuningAnalysis, id string, opcRequestID string) {
	status := &resource.Status.OsokStatus
	servicemanager.SetOpcRequestID(status, opcRequestID)
	if id != "" {
		status.Ocid = shared.OCID(id)
	}
	now := metav1.Now()
	if status.CreatedAt == nil && id != "" {
		status.CreatedAt = &now
	}
	status.UpdatedAt = &now
	status.Message = fmt.Sprintf("OCI %s %s is active", performanceTuningAnalysisKind, id)
	status.Reason = string(shared.Active)
	servicemanager.ClearAsyncOperation(status)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Active, corev1.ConditionTrue, "", status.Message, c.log)
}

func (c *performanceTuningAnalysisRuntimeClient) markTerminating(resource *jmsutilsv1beta1.PerformanceTuningAnalysis, message string) {
	status := &resource.Status.OsokStatus
	now := metav1.Now()
	status.UpdatedAt = &now
	status.Message = strings.TrimSpace(message)
	status.Reason = string(shared.Terminating)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Terminating, corev1.ConditionTrue, "", status.Message, c.log)
}

func (c *performanceTuningAnalysisRuntimeClient) markDeleted(resource *jmsutilsv1beta1.PerformanceTuningAnalysis, message string) {
	status := &resource.Status.OsokStatus
	now := metav1.Now()
	status.DeletedAt = &now
	status.UpdatedAt = &now
	status.Message = strings.TrimSpace(message)
	status.Reason = string(shared.Terminating)
	servicemanager.ClearAsyncOperation(status)
	*status = util.UpdateOSOKStatusCondition(*status, shared.Terminating, corev1.ConditionTrue, "", status.Message, c.log)
}

func (c *performanceTuningAnalysisRuntimeClient) fail(resource *jmsutilsv1beta1.PerformanceTuningAnalysis, err error) error {
	if resource == nil || err == nil {
		return err
	}
	status := &resource.Status.OsokStatus
	servicemanager.RecordErrorOpcRequestID(status, err)
	status.Message = err.Error()
	status.Reason = string(shared.Failed)
	now := metav1.Now()
	status.UpdatedAt = &now
	if status.Async.Current != nil {
		current := *status.Async.Current
		current.NormalizedClass = shared.OSOKAsyncClassFailed
		current.Message = err.Error()
		current.UpdatedAt = &now
		servicemanager.ApplyAsyncOperation(status, &current, c.log)
		return err
	}
	*status = util.UpdateOSOKStatusCondition(*status, shared.Failed, corev1.ConditionFalse, "", err.Error(), c.log)
	return err
}

func performanceTuningAnalysisIDFromWorkRequest(workRequest jmsutilssdk.WorkRequest) string {
	for _, resource := range workRequest.Resources {
		identifier := stringValue(resource.Identifier)
		if identifier == "" {
			continue
		}
		if isPerformanceTuningAnalysisWorkRequestResource(resource) {
			return identifier
		}
	}
	return ""
}

func isPerformanceTuningAnalysisWorkRequestResource(resource jmsutilssdk.WorkRequestResource) bool {
	return normalizedPerformanceTuningAnalysisResourceToken(resource.EntityType) ||
		normalizedPerformanceTuningAnalysisResourceToken(resource.EntityUri)
}

func normalizedPerformanceTuningAnalysisResourceToken(value *string) bool {
	normalized := strings.ToLower(strings.TrimSpace(stringValue(value)))
	normalized = strings.NewReplacer("_", "", "-", "", " ", "", ".", "", "/", "").Replace(normalized)
	return strings.Contains(normalized, "performancetuninganalysis")
}

func currentPerformanceTuningAnalysisWorkRequestID(resource *jmsutilsv1beta1.PerformanceTuningAnalysis) string {
	if resource == nil {
		return ""
	}
	if resource.Status.OsokStatus.Async.Current != nil &&
		resource.Status.OsokStatus.Async.Current.Phase == shared.OSOKAsyncPhaseCreate {
		if workRequestID := strings.TrimSpace(resource.Status.OsokStatus.Async.Current.WorkRequestID); workRequestID != "" {
			return workRequestID
		}
	}
	return strings.TrimSpace(resource.Status.WorkRequestId)
}

func performanceTuningAnalysisRetryToken(resource *jmsutilsv1beta1.PerformanceTuningAnalysis, identity performanceTuningAnalysisIdentity) string {
	seed := strings.Join([]string{
		string(resource.UID),
		resource.Namespace,
		resource.Name,
		identity.compartmentID,
		identity.analysisProjectName,
		identity.artifactObjectStoragePath,
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("osok-pta-%x", sum[:16])
}

func isPerformanceTuningAnalysisUnambiguousNotFound(err error) bool {
	statusCode, code, ok := performanceTuningAnalysisServiceError(err)
	return ok && statusCode == 404 && code == errorutil.NotFound
}

func isPerformanceTuningAnalysisAmbiguousNotFound(err error) bool {
	statusCode, code, ok := performanceTuningAnalysisServiceError(err)
	return ok && statusCode == 404 && code == errorutil.NotAuthorizedOrNotFound
}

func performanceTuningAnalysisServiceError(err error) (int, string, bool) {
	var serviceErr common.ServiceError
	if errors.As(err, &serviceErr) {
		return serviceErr.GetHTTPStatusCode(), strings.TrimSpace(serviceErr.GetCode()), true
	}
	return 0, "", false
}

func projectPerformanceTuningAnalysisPrincipal(principal *jmsutilssdk.Principal) jmsutilsv1beta1.PerformanceTuningAnalysisCreatedBy {
	if principal == nil {
		return jmsutilsv1beta1.PerformanceTuningAnalysisCreatedBy{}
	}
	return jmsutilsv1beta1.PerformanceTuningAnalysisCreatedBy{
		Id:          stringValue(principal.Id),
		DisplayName: stringValue(principal.DisplayName),
	}
}

func sdkTimeString(value *common.SDKTime) string {
	if value == nil || value.IsZero() {
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

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func cloneFloat32Ptr(value *float32) *float32 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func annotationValue(annotations map[string]string, key string) string {
	if len(annotations) == 0 {
		return ""
	}
	return strings.TrimSpace(annotations[key])
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
