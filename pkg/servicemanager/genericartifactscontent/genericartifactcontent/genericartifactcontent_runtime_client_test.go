/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package genericartifactcontent

import (
	"context"
	"strings"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	genericartifactscontentsdk "github.com/oracle/oci-go-sdk/v65/genericartifactscontent"
	genericartifactscontentv1beta1 "github.com/oracle/oci-service-operator/api/genericartifactscontent/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil"
	"github.com/oracle/oci-service-operator/pkg/errorutil/errortest"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

const testGenericArtifactContentID = "ocid1.genericartifact.oc1..content"

type fakeGenericArtifactContentOCIClient struct {
	getFn       func(context.Context, genericartifactscontentsdk.GetGenericArtifactContentRequest) (genericartifactscontentsdk.GetGenericArtifactContentResponse, error)
	getRequests []genericartifactscontentsdk.GetGenericArtifactContentRequest
}

func (f *fakeGenericArtifactContentOCIClient) GetGenericArtifactContent(
	ctx context.Context,
	request genericartifactscontentsdk.GetGenericArtifactContentRequest,
) (genericartifactscontentsdk.GetGenericArtifactContentResponse, error) {
	f.getRequests = append(f.getRequests, request)
	if f.getFn != nil {
		return f.getFn(ctx, request)
	}
	return genericartifactscontentsdk.GetGenericArtifactContentResponse{
		Content:      newTrackedReadCloser("artifact content"),
		OpcRequestId: common.String("opc-get"),
	}, nil
}

type trackedReadCloser struct {
	*strings.Reader
	closed bool
}

func newTrackedReadCloser(value string) *trackedReadCloser {
	return &trackedReadCloser{Reader: strings.NewReader(value)}
}

func (r *trackedReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestGenericArtifactContentCreateOrUpdateRequiresTrackedArtifactID(t *testing.T) {
	t.Parallel()

	fake := &fakeGenericArtifactContentOCIClient{
		getFn: func(context.Context, genericartifactscontentsdk.GetGenericArtifactContentRequest) (genericartifactscontentsdk.GetGenericArtifactContentResponse, error) {
			t.Fatal("GetGenericArtifactContent() should not be called without status.status.ocid")
			return genericartifactscontentsdk.GetGenericArtifactContentResponse{}, nil
		},
	}
	client := newGenericArtifactContentRuntimeClientWithOCIClient(fake)
	resource := newGenericArtifactContentResource("")

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want missing tracked ID error")
	}
	if !strings.Contains(err.Error(), "requires status.status.ocid") {
		t.Fatalf("CreateOrUpdate() error = %v, want status.status.ocid requirement", err)
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = true, want false")
	}
	requireGenericArtifactContentCallCount(t, "GetGenericArtifactContent()", len(fake.getRequests), 0)
	requireGenericArtifactContentCondition(t, resource, shared.Failed)
}

func TestGenericArtifactContentCreateOrUpdateReadsTrackedContentWithoutProjectingBody(t *testing.T) {
	t.Parallel()

	body := newTrackedReadCloser("do-not-project")
	fake := &fakeGenericArtifactContentOCIClient{
		getFn: func(_ context.Context, request genericartifactscontentsdk.GetGenericArtifactContentRequest) (genericartifactscontentsdk.GetGenericArtifactContentResponse, error) {
			requireGenericArtifactContentStringPtr(t, "artifactId", request.ArtifactId, testGenericArtifactContentID)
			return genericartifactscontentsdk.GetGenericArtifactContentResponse{
				Content:      body,
				OpcRequestId: common.String("opc-get"),
			}, nil
		},
	}
	client := newGenericArtifactContentRuntimeClientWithOCIClient(fake)
	resource := newGenericArtifactContentResource(testGenericArtifactContentID)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	requireGenericArtifactContentNoError(t, "CreateOrUpdate()", err)
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = false, want true")
	}
	if response.ShouldRequeue {
		t.Fatal("CreateOrUpdate() ShouldRequeue = true, want false")
	}
	if !body.closed {
		t.Fatal("response content body was not closed")
	}
	requireGenericArtifactContentCallCount(t, "GetGenericArtifactContent()", len(fake.getRequests), 1)
	requireGenericArtifactContentString(t, "status.status.ocid", string(resource.Status.OsokStatus.Ocid), testGenericArtifactContentID)
	requireGenericArtifactContentString(t, "status.status.opcRequestId", resource.Status.OsokStatus.OpcRequestID, "opc-get")
	requireGenericArtifactContentString(t, "status.status.message", resource.Status.OsokStatus.Message, genericArtifactContentActiveMessage)
	if resource.Status.OsokStatus.CreatedAt == nil {
		t.Fatal("status.status.createdAt = nil, want timestamp")
	}
	requireGenericArtifactContentCondition(t, resource, shared.Active)
}

func TestGenericArtifactContentCreateOrUpdateNoOpRereadsTrackedContent(t *testing.T) {
	t.Parallel()

	fake := &fakeGenericArtifactContentOCIClient{}
	client := newGenericArtifactContentRuntimeClientWithOCIClient(fake)
	resource := newGenericArtifactContentResource(testGenericArtifactContentID)

	for i := 0; i < 2; i++ {
		response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
		requireGenericArtifactContentNoError(t, "CreateOrUpdate()", err)
		if !response.IsSuccessful {
			t.Fatal("CreateOrUpdate() IsSuccessful = false, want true")
		}
	}
	requireGenericArtifactContentCallCount(t, "GetGenericArtifactContent()", len(fake.getRequests), 2)
	requireGenericArtifactContentCondition(t, resource, shared.Active)
	requireGenericArtifactContentString(t, "status.status.ocid", string(resource.Status.OsokStatus.Ocid), testGenericArtifactContentID)
}

func TestGenericArtifactContentCreateOrUpdateRecordsOCIErrorRequestID(t *testing.T) {
	t.Parallel()

	ociErr := errortest.NewServiceError(500, errorutil.InternalServerError, "read failed")
	ociErr.OpcRequestID = "opc-read-error"
	fake := &fakeGenericArtifactContentOCIClient{
		getFn: func(context.Context, genericartifactscontentsdk.GetGenericArtifactContentRequest) (genericartifactscontentsdk.GetGenericArtifactContentResponse, error) {
			return genericartifactscontentsdk.GetGenericArtifactContentResponse{}, ociErr
		},
	}
	client := newGenericArtifactContentRuntimeClientWithOCIClient(fake)
	resource := newGenericArtifactContentResource(testGenericArtifactContentID)

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err == nil {
		t.Fatal("CreateOrUpdate() error = nil, want OCI read error")
	}
	if response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = true, want false")
	}
	requireGenericArtifactContentString(t, "status.status.opcRequestId", resource.Status.OsokStatus.OpcRequestID, "opc-read-error")
	requireGenericArtifactContentCondition(t, resource, shared.Failed)
}

func TestGenericArtifactContentDeleteNoTrackedIDReleasesFinalizerWithoutOCI(t *testing.T) {
	t.Parallel()

	fake := &fakeGenericArtifactContentOCIClient{
		getFn: func(context.Context, genericartifactscontentsdk.GetGenericArtifactContentRequest) (genericartifactscontentsdk.GetGenericArtifactContentResponse, error) {
			t.Fatal("GetGenericArtifactContent() should not be called without status.status.ocid")
			return genericartifactscontentsdk.GetGenericArtifactContentResponse{}, nil
		},
	}
	client := newGenericArtifactContentRuntimeClientWithOCIClient(fake)
	resource := newGenericArtifactContentResource("")

	deleted, err := client.Delete(context.Background(), resource)
	requireGenericArtifactContentNoError(t, "Delete()", err)
	if !deleted {
		t.Fatal("Delete() deleted = false, want true")
	}
	requireGenericArtifactContentCallCount(t, "GetGenericArtifactContent()", len(fake.getRequests), 0)
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("status.status.deletedAt = nil, want timestamp")
	}
	requireGenericArtifactContentCondition(t, resource, shared.Terminating)
}

func TestGenericArtifactContentDeleteConfirmsReadableContentThenReleasesFinalizerWithoutDelete(t *testing.T) {
	t.Parallel()

	body := newTrackedReadCloser("still-readable")
	fake := &fakeGenericArtifactContentOCIClient{
		getFn: func(_ context.Context, request genericartifactscontentsdk.GetGenericArtifactContentRequest) (genericartifactscontentsdk.GetGenericArtifactContentResponse, error) {
			requireGenericArtifactContentStringPtr(t, "artifactId", request.ArtifactId, testGenericArtifactContentID)
			return genericartifactscontentsdk.GetGenericArtifactContentResponse{
				Content:      body,
				OpcRequestId: common.String("opc-delete-read"),
			}, nil
		},
	}
	client := newGenericArtifactContentRuntimeClientWithOCIClient(fake)
	resource := newGenericArtifactContentResource(testGenericArtifactContentID)

	deleted, err := client.Delete(context.Background(), resource)
	requireGenericArtifactContentNoError(t, "Delete()", err)
	if !deleted {
		t.Fatal("Delete() deleted = false, want true")
	}
	if !body.closed {
		t.Fatal("response content body was not closed")
	}
	requireGenericArtifactContentCallCount(t, "GetGenericArtifactContent()", len(fake.getRequests), 1)
	requireGenericArtifactContentString(t, "status.status.opcRequestId", resource.Status.OsokStatus.OpcRequestID, "opc-delete-read")
	requireGenericArtifactContentString(t, "status.status.message", resource.Status.OsokStatus.Message, genericArtifactContentDeleteNoopMessage)
	requireGenericArtifactContentCondition(t, resource, shared.Terminating)
}

func TestGenericArtifactContentDeleteConfirmsUnambiguousNotFound(t *testing.T) {
	t.Parallel()

	ociErr := errortest.NewServiceError(404, errorutil.NotFound, "content deleted")
	ociErr.OpcRequestID = "opc-delete-not-found"
	fake := &fakeGenericArtifactContentOCIClient{
		getFn: func(context.Context, genericartifactscontentsdk.GetGenericArtifactContentRequest) (genericartifactscontentsdk.GetGenericArtifactContentResponse, error) {
			return genericartifactscontentsdk.GetGenericArtifactContentResponse{}, ociErr
		},
	}
	client := newGenericArtifactContentRuntimeClientWithOCIClient(fake)
	resource := newGenericArtifactContentResource(testGenericArtifactContentID)

	deleted, err := client.Delete(context.Background(), resource)
	requireGenericArtifactContentNoError(t, "Delete()", err)
	if !deleted {
		t.Fatal("Delete() deleted = false, want true")
	}
	requireGenericArtifactContentString(t, "status.status.opcRequestId", resource.Status.OsokStatus.OpcRequestID, "opc-delete-not-found")
	requireGenericArtifactContentString(t, "status.status.message", resource.Status.OsokStatus.Message, genericArtifactContentDeleteMissingMessage)
	requireGenericArtifactContentCondition(t, resource, shared.Terminating)
}

func TestGenericArtifactContentDeleteKeepsFinalizerOnAmbiguousNotFound(t *testing.T) {
	t.Parallel()

	ociErr := errortest.NewServiceError(404, errorutil.NotAuthorizedOrNotFound, "ambiguous")
	ociErr.OpcRequestID = "opc-delete-auth"
	fake := &fakeGenericArtifactContentOCIClient{
		getFn: func(context.Context, genericartifactscontentsdk.GetGenericArtifactContentRequest) (genericartifactscontentsdk.GetGenericArtifactContentResponse, error) {
			return genericartifactscontentsdk.GetGenericArtifactContentResponse{}, ociErr
		},
	}
	client := newGenericArtifactContentRuntimeClientWithOCIClient(fake)
	resource := newGenericArtifactContentResource(testGenericArtifactContentID)

	deleted, err := client.Delete(context.Background(), resource)
	if err == nil {
		t.Fatal("Delete() error = nil, want ambiguous NotAuthorizedOrNotFound")
	}
	if !strings.Contains(err.Error(), "ambiguous 404 NotAuthorizedOrNotFound") {
		t.Fatalf("Delete() error = %v, want ambiguous NotAuthorizedOrNotFound", err)
	}
	if deleted {
		t.Fatal("Delete() deleted = true, want false")
	}
	requireGenericArtifactContentString(t, "status.status.opcRequestId", resource.Status.OsokStatus.OpcRequestID, "opc-delete-auth")
	requireGenericArtifactContentCondition(t, resource, shared.Failed)
}

func newGenericArtifactContentResource(artifactID string) *genericartifactscontentv1beta1.GenericArtifactContent {
	resource := &genericartifactscontentv1beta1.GenericArtifactContent{}
	if artifactID != "" {
		resource.Status.OsokStatus.Ocid = shared.OCID(artifactID)
	}
	return resource
}

func requireGenericArtifactContentNoError(t *testing.T, operation string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s error = %v", operation, err)
	}
}

func requireGenericArtifactContentCallCount(t *testing.T, operation string, got int, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("%s calls = %d, want %d", operation, got, want)
	}
}

func requireGenericArtifactContentString(t *testing.T, name string, got string, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}

func requireGenericArtifactContentStringPtr(t *testing.T, name string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %q", name, want)
	}
	requireGenericArtifactContentString(t, name, *got, want)
}

func requireGenericArtifactContentCondition(
	t *testing.T,
	resource *genericartifactscontentv1beta1.GenericArtifactContent,
	want shared.OSOKConditionType,
) {
	t.Helper()
	if len(resource.Status.OsokStatus.Conditions) == 0 {
		t.Fatalf("status.status.conditions is empty, want %s", want)
	}
	got := resource.Status.OsokStatus.Conditions[len(resource.Status.OsokStatus.Conditions)-1].Type
	if got != want {
		t.Fatalf("latest condition = %s, want %s", got, want)
	}
}
