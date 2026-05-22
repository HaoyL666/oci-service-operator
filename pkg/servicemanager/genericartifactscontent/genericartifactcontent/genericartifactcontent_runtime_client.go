/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package genericartifactcontent

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	genericartifactscontentsdk "github.com/oracle/oci-go-sdk/v65/genericartifactscontent"
	genericartifactscontentv1beta1 "github.com/oracle/oci-service-operator/api/genericartifactscontent/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	genericArtifactContentKind = "GenericArtifactContent"

	genericArtifactContentActiveMessage        = "OCI GenericArtifactContent content is readable"
	genericArtifactContentNoTrackedIDMessage   = "GenericArtifactContent is read-only and requires status.status.ocid to bind existing OCI artifact content"
	genericArtifactContentDeleteNoopMessage    = "OCI GenericArtifactContent content is read-only; deleting the Kubernetes resource leaves OCI artifact content unchanged"
	genericArtifactContentDeleteMissingMessage = "OCI GenericArtifactContent content no longer exists"
)

type genericArtifactContentOCIClient interface {
	GetGenericArtifactContent(context.Context, genericartifactscontentsdk.GetGenericArtifactContentRequest) (genericartifactscontentsdk.GetGenericArtifactContentResponse, error)
}

type genericArtifactContentRuntimeClient struct {
	client  genericArtifactContentOCIClient
	initErr error
	log     loggerutil.OSOKLogger
}

func init() {
	registerGenericArtifactContentRuntimeHooksMutator(func(manager *GenericArtifactContentServiceManager, hooks *GenericArtifactContentRuntimeHooks) {
		client, initErr := newGenericArtifactContentSDKClient(manager)
		hooks.Semantics = newGenericArtifactContentRuntimeSemantics()
		hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, func(_ GenericArtifactContentServiceClient) GenericArtifactContentServiceClient {
			return newGenericArtifactContentRuntimeClient(manager, client, initErr)
		})
	})
}

func newGenericArtifactContentSDKClient(manager *GenericArtifactContentServiceManager) (genericArtifactContentOCIClient, error) {
	if manager == nil {
		return nil, fmt.Errorf("%s service manager is nil", genericArtifactContentKind)
	}
	client, err := genericartifactscontentsdk.NewGenericArtifactsContentClientWithConfigurationProvider(manager.Provider)
	if err != nil {
		return nil, fmt.Errorf("initialize %s OCI client: %w", genericArtifactContentKind, err)
	}
	return client, nil
}

func newGenericArtifactContentRuntimeClient(
	manager *GenericArtifactContentServiceManager,
	client genericArtifactContentOCIClient,
	initErr error,
) GenericArtifactContentServiceClient {
	runtimeClient := &genericArtifactContentRuntimeClient{
		client:  client,
		initErr: initErr,
	}
	if manager != nil {
		runtimeClient.log = manager.Log
	}
	return runtimeClient
}

func newGenericArtifactContentRuntimeClientWithOCIClient(client genericArtifactContentOCIClient) GenericArtifactContentServiceClient {
	return &genericArtifactContentRuntimeClient{client: client}
}

func newGenericArtifactContentRuntimeSemantics() *generatedruntime.Semantics {
	return &generatedruntime.Semantics{
		FormalService:     "genericartifactscontent",
		FormalSlug:        "genericartifactcontent",
		StatusProjection:  "required",
		SecretSideEffects: "none",
		FinalizerPolicy:   "read-only-bind-release",
		Lifecycle: generatedruntime.LifecycleSemantics{
			ActiveStates: []string{string(genericartifactscontentsdk.GenericArtifactLifecycleStateAvailable)},
		},
		Hooks: generatedruntime.HookSet{
			Create: []generatedruntime.Hook{{Helper: "resource-local read-only bind", EntityType: genericArtifactContentKind, Action: "GetGenericArtifactContent"}},
			Delete: []generatedruntime.Hook{{Helper: "resource-local read-only cleanup", EntityType: genericArtifactContentKind, Action: "GetGenericArtifactContent"}},
		},
		Unsupported: []generatedruntime.UnsupportedSemantic{{
			Category:      "sdk-surface",
			StopCondition: "the genericartifactscontent SDK exposes Create/Update/Delete/List operations for GenericArtifactContent",
		}, {
			Category:      "crd-shape",
			StopCondition: "GenericArtifactContent spec exposes artifactId or another bind identity instead of requiring existing status.status.ocid",
		}},
	}
}

func (c *genericArtifactContentRuntimeClient) CreateOrUpdate(
	ctx context.Context,
	resource *genericartifactscontentv1beta1.GenericArtifactContent,
	_ ctrl.Request,
) (servicemanager.OSOKResponse, error) {
	if err := c.validateConfigured(); err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
	}

	artifactID, err := genericArtifactContentTrackedID(resource)
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
	}

	response, err := c.client.GetGenericArtifactContent(ctx, genericartifactscontentsdk.GetGenericArtifactContentRequest{
		ArtifactId: common.String(artifactID),
	})
	if err != nil {
		return servicemanager.OSOKResponse{IsSuccessful: false}, c.fail(resource, err)
	}
	closeGenericArtifactContentBody(response.Content)
	c.markActive(resource, artifactID, response)
	return servicemanager.OSOKResponse{IsSuccessful: true}, nil
}

func (c *genericArtifactContentRuntimeClient) Delete(
	ctx context.Context,
	resource *genericartifactscontentv1beta1.GenericArtifactContent,
) (bool, error) {
	if err := c.validateConfigured(); err != nil {
		return false, c.fail(resource, err)
	}
	if resource == nil {
		return false, fmt.Errorf("%s resource is nil", genericArtifactContentKind)
	}

	artifactID := strings.TrimSpace(string(resource.Status.OsokStatus.Ocid))
	if artifactID == "" {
		c.markDeleted(resource, "no OCI GenericArtifactContent content was tracked")
		return true, nil
	}

	response, err := c.client.GetGenericArtifactContent(ctx, genericartifactscontentsdk.GetGenericArtifactContentRequest{
		ArtifactId: common.String(artifactID),
	})
	if err != nil {
		servicemanager.RecordErrorOpcRequestID(&resource.Status.OsokStatus, err)
		classification := errorutil.ClassifyDeleteError(err)
		if classification.IsUnambiguousNotFound() {
			c.markDeleted(resource, genericArtifactContentDeleteMissingMessage)
			return true, nil
		}
		if classification.IsAuthShapedNotFound() {
			return false, c.fail(resource, fmt.Errorf("%s delete confirmation returned ambiguous 404 NotAuthorizedOrNotFound; keeping finalizer until readback is unambiguous: %w", genericArtifactContentKind, err))
		}
		return false, c.fail(resource, err)
	}

	closeGenericArtifactContentBody(response.Content)
	servicemanager.RecordResponseOpcRequestID(&resource.Status.OsokStatus, response)
	c.markDeleted(resource, genericArtifactContentDeleteNoopMessage)
	return true, nil
}

func (c *genericArtifactContentRuntimeClient) validateConfigured() error {
	if c == nil {
		return fmt.Errorf("%s runtime client is nil", genericArtifactContentKind)
	}
	if c.initErr != nil {
		return c.initErr
	}
	if c.client == nil {
		return fmt.Errorf("%s OCI client is not configured", genericArtifactContentKind)
	}
	return nil
}

func genericArtifactContentTrackedID(resource *genericartifactscontentv1beta1.GenericArtifactContent) (string, error) {
	if resource == nil {
		return "", fmt.Errorf("%s resource is nil", genericArtifactContentKind)
	}
	artifactID := strings.TrimSpace(string(resource.Status.OsokStatus.Ocid))
	if artifactID == "" {
		return "", fmt.Errorf("%s; use GenericArtifactContentByPath for artifact uploads", genericArtifactContentNoTrackedIDMessage)
	}
	return artifactID, nil
}

func closeGenericArtifactContentBody(content io.Closer) {
	if content != nil {
		_ = content.Close()
	}
}

func (c *genericArtifactContentRuntimeClient) markActive(
	resource *genericartifactscontentv1beta1.GenericArtifactContent,
	artifactID string,
	response genericartifactscontentsdk.GetGenericArtifactContentResponse,
) {
	if resource == nil {
		return
	}
	status := &resource.Status.OsokStatus
	servicemanager.RecordResponseOpcRequestID(status, response)
	status.Ocid = shared.OCID(artifactID)
	status.Async.Current = nil
	status.Message = genericArtifactContentActiveMessage
	status.Reason = string(shared.Active)
	now := metav1.Now()
	if status.CreatedAt == nil {
		status.CreatedAt = &now
	}
	status.UpdatedAt = &now
	*status = util.UpdateOSOKStatusCondition(*status, shared.Active, v1.ConditionTrue, "", status.Message, c.log)
}

func (c *genericArtifactContentRuntimeClient) markDeleted(
	resource *genericartifactscontentv1beta1.GenericArtifactContent,
	message string,
) {
	status := &resource.Status.OsokStatus
	status.Async.Current = nil
	status.Message = message
	status.Reason = string(shared.Terminating)
	now := metav1.Now()
	status.DeletedAt = &now
	status.UpdatedAt = &now
	*status = util.UpdateOSOKStatusCondition(*status, shared.Terminating, v1.ConditionTrue, "", message, c.log)
}

func (c *genericArtifactContentRuntimeClient) fail(
	resource *genericartifactscontentv1beta1.GenericArtifactContent,
	err error,
) error {
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
		status.Async.Current = &current
		return err
	}
	*status = util.UpdateOSOKStatusCondition(*status, shared.Failed, v1.ConditionFalse, "", err.Error(), c.log)
	return err
}
