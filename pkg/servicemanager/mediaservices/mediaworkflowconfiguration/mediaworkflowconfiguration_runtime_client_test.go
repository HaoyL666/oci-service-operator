package mediaworkflowconfiguration

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/oracle/oci-go-sdk/v65/common"
	mediaservicessdk "github.com/oracle/oci-go-sdk/v65/mediaservices"
	mediaservicesv1beta1 "github.com/oracle/oci-service-operator/api/mediaservices/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	ctrl "sigs.k8s.io/controller-runtime"
)

const testMediaWorkflowConfigurationID = "ocid1.mediaworkflowconfiguration.oc1..runtime"

type fakeMediaWorkflowConfigurationOCIClient struct {
	createFn func(context.Context, mediaservicessdk.CreateMediaWorkflowConfigurationRequest) (mediaservicessdk.CreateMediaWorkflowConfigurationResponse, error)
	getFn    func(context.Context, mediaservicessdk.GetMediaWorkflowConfigurationRequest) (mediaservicessdk.GetMediaWorkflowConfigurationResponse, error)
	listFn   func(context.Context, mediaservicessdk.ListMediaWorkflowConfigurationsRequest) (mediaservicessdk.ListMediaWorkflowConfigurationsResponse, error)
	updateFn func(context.Context, mediaservicessdk.UpdateMediaWorkflowConfigurationRequest) (mediaservicessdk.UpdateMediaWorkflowConfigurationResponse, error)
	deleteFn func(context.Context, mediaservicessdk.DeleteMediaWorkflowConfigurationRequest) (mediaservicessdk.DeleteMediaWorkflowConfigurationResponse, error)
}

func (f *fakeMediaWorkflowConfigurationOCIClient) CreateMediaWorkflowConfiguration(
	ctx context.Context,
	req mediaservicessdk.CreateMediaWorkflowConfigurationRequest,
) (mediaservicessdk.CreateMediaWorkflowConfigurationResponse, error) {
	if f.createFn != nil {
		return f.createFn(ctx, req)
	}
	return mediaservicessdk.CreateMediaWorkflowConfigurationResponse{}, nil
}

func (f *fakeMediaWorkflowConfigurationOCIClient) GetMediaWorkflowConfiguration(
	ctx context.Context,
	req mediaservicessdk.GetMediaWorkflowConfigurationRequest,
) (mediaservicessdk.GetMediaWorkflowConfigurationResponse, error) {
	if f.getFn != nil {
		return f.getFn(ctx, req)
	}
	return mediaservicessdk.GetMediaWorkflowConfigurationResponse{}, nil
}

func (f *fakeMediaWorkflowConfigurationOCIClient) ListMediaWorkflowConfigurations(
	ctx context.Context,
	req mediaservicessdk.ListMediaWorkflowConfigurationsRequest,
) (mediaservicessdk.ListMediaWorkflowConfigurationsResponse, error) {
	if f.listFn != nil {
		return f.listFn(ctx, req)
	}
	return mediaservicessdk.ListMediaWorkflowConfigurationsResponse{}, nil
}

func (f *fakeMediaWorkflowConfigurationOCIClient) UpdateMediaWorkflowConfiguration(
	ctx context.Context,
	req mediaservicessdk.UpdateMediaWorkflowConfigurationRequest,
) (mediaservicessdk.UpdateMediaWorkflowConfigurationResponse, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, req)
	}
	return mediaservicessdk.UpdateMediaWorkflowConfigurationResponse{}, nil
}

func (f *fakeMediaWorkflowConfigurationOCIClient) DeleteMediaWorkflowConfiguration(
	ctx context.Context,
	req mediaservicessdk.DeleteMediaWorkflowConfigurationRequest,
) (mediaservicessdk.DeleteMediaWorkflowConfigurationResponse, error) {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, req)
	}
	return mediaservicessdk.DeleteMediaWorkflowConfigurationResponse{}, nil
}

func newMediaWorkflowConfigurationTestClient(fake *fakeMediaWorkflowConfigurationOCIClient) MediaWorkflowConfigurationServiceClient {
	return newMediaWorkflowConfigurationServiceClientWithOCIClient(
		loggerutil.OSOKLogger{Logger: logr.Discard()},
		fake,
	)
}

func mediaWorkflowConfigurationJSONValue(raw string) shared.JSONValue {
	return shared.JSONValue{Raw: []byte(raw)}
}

func newMediaWorkflowConfigurationTestResource() *mediaservicesv1beta1.MediaWorkflowConfiguration {
	return &mediaservicesv1beta1.MediaWorkflowConfiguration{
		Spec: mediaservicesv1beta1.MediaWorkflowConfigurationSpec{
			DisplayName:   "configuration-alpha",
			CompartmentId: "ocid1.compartment.oc1..example",
			Parameters: map[string]shared.JSONValue{
				"transcode":  mediaWorkflowConfigurationJSONValue(`{"preset":"HD","bitrateKbps":6000}`),
				"thumbnails": mediaWorkflowConfigurationJSONValue(`{"enabled":true}`),
			},
			FreeformTags: map[string]string{"env": "dev"},
			DefinedTags: map[string]shared.MapValue{
				"Operations": {"CostCenter": "42"},
			},
			Locks: []mediaservicesv1beta1.MediaWorkflowConfigurationLock{{
				Type:              "DELETE",
				CompartmentId:     "ocid1.compartment.oc1..example",
				RelatedResourceId: "ocid1.locksource.oc1..example",
				Message:           "managed lock",
			}},
		},
	}
}

func trackMediaWorkflowConfiguration(resource *mediaservicesv1beta1.MediaWorkflowConfiguration, mediaWorkflowConfigurationID string) {
	resource.Status.Id = mediaWorkflowConfigurationID
	resource.Status.OsokStatus.Ocid = shared.OCID(mediaWorkflowConfigurationID)
}

func observedMediaWorkflowConfigurationFromSpec(
	id string,
	spec mediaservicesv1beta1.MediaWorkflowConfigurationSpec,
	state mediaservicessdk.MediaWorkflowConfigurationLifecycleStateEnum,
) mediaservicessdk.MediaWorkflowConfiguration {
	parameters, err := mediaWorkflowConfigurationParametersFromSpec(spec.Parameters)
	if err != nil {
		panic(err)
	}
	definedTags, err := mediaWorkflowConfigurationDefinedTagsFromSpec(spec.DefinedTags)
	if err != nil {
		panic(err)
	}
	return mediaservicessdk.MediaWorkflowConfiguration{
		Id:             common.String(id),
		DisplayName:    optionalString(spec.DisplayName),
		CompartmentId:  common.String(spec.CompartmentId),
		Parameters:     parameters,
		LifecycleState: state,
		Locks:          mediaWorkflowConfigurationLocksFromSpec(spec.Locks),
		FreeformTags:   cloneStringMap(spec.FreeformTags),
		DefinedTags:    definedTags,
	}
}

func mediaWorkflowConfigurationLocksFromSpec(spec []mediaservicesv1beta1.MediaWorkflowConfigurationLock) []mediaservicessdk.ResourceLock {
	converted := make([]mediaservicessdk.ResourceLock, 0, len(spec))
	for _, lock := range spec {
		converted = append(converted, mediaservicessdk.ResourceLock{
			Type:              mediaservicessdk.ResourceLockTypeEnum(lock.Type),
			CompartmentId:     optionalString(lock.CompartmentId),
			RelatedResourceId: optionalString(lock.RelatedResourceId),
			Message:           optionalString(lock.Message),
		})
	}
	return converted
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return common.String(value)
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func mediaWorkflowConfigurationSerializedRequestBody(t *testing.T, request any, method string, path string) string {
	t.Helper()

	ociRequest, ok := request.(interface {
		HTTPRequest(string, string, *common.OCIReadSeekCloser, map[string]string) (http.Request, error)
	})
	if !ok {
		t.Fatalf("request type %T does not implement HTTPRequest", request)
	}

	httpRequest, err := ociRequest.HTTPRequest(method, path, nil, nil)
	if err != nil {
		t.Fatalf("HTTPRequest(%T) error = %v", request, err)
	}
	if httpRequest.Body == nil {
		return ""
	}
	payload, err := io.ReadAll(httpRequest.Body)
	if err != nil {
		t.Fatalf("io.ReadAll(%T body) error = %v", request, err)
	}
	return string(payload)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestApplyMediaWorkflowConfigurationRuntimeHooksUsesReviewedContract(t *testing.T) {
	t.Parallel()

	hooks := newMediaWorkflowConfigurationDefaultRuntimeHooks(mediaservicessdk.MediaServicesClient{})
	applyMediaWorkflowConfigurationRuntimeHooks(&hooks)

	if hooks.Semantics == nil {
		t.Fatal("hooks.Semantics = nil, want reviewed semantics")
	}
	if len(hooks.Semantics.Lifecycle.ProvisioningStates) != 0 {
		t.Fatalf("provisioning states = %#v, want none for synchronous contract", hooks.Semantics.Lifecycle.ProvisioningStates)
	}
	if len(hooks.Semantics.Lifecycle.UpdatingStates) != 0 {
		t.Fatalf("updating states = %#v, want none for synchronous contract", hooks.Semantics.Lifecycle.UpdatingStates)
	}
	if !reflect.DeepEqual(hooks.List.Fields, mediaWorkflowConfigurationListFields()) {
		t.Fatalf("list fields = %#v, want %#v", hooks.List.Fields, mediaWorkflowConfigurationListFields())
	}
	if !reflect.DeepEqual(hooks.Update.Fields, mediaWorkflowConfigurationUpdateFields()) {
		t.Fatalf("update fields = %#v, want %#v", hooks.Update.Fields, mediaWorkflowConfigurationUpdateFields())
	}
	if !reflect.DeepEqual(hooks.Delete.Fields, mediaWorkflowConfigurationDeleteFields()) {
		t.Fatalf("delete fields = %#v, want %#v", hooks.Delete.Fields, mediaWorkflowConfigurationDeleteFields())
	}
	if hooks.BuildCreateBody == nil {
		t.Fatal("hooks.BuildCreateBody = nil, want create body sanitizer")
	}
	if hooks.BuildUpdateBody == nil {
		t.Fatal("hooks.BuildUpdateBody = nil, want reviewed update builder")
	}
	if hooks.Identity.GuardExistingBeforeCreate == nil {
		t.Fatal("hooks.Identity.GuardExistingBeforeCreate = nil, want bounded pre-create reuse")
	}
	if hooks.ParityHooks.NormalizeDesiredState == nil {
		t.Fatal("hooks.ParityHooks.NormalizeDesiredState = nil, want lock normalization hook")
	}
	if hooks.ParityHooks.ValidateCreateOnlyDrift == nil {
		t.Fatal("hooks.ParityHooks.ValidateCreateOnlyDrift = nil, want create-only drift guard")
	}
	if got := hooks.Semantics.CreateFollowUp.Strategy; got != "read-after-write" {
		t.Fatalf("create follow-up = %q, want %q", got, "read-after-write")
	}
	if got := hooks.Semantics.UpdateFollowUp.Strategy; got != "read-after-write" {
		t.Fatalf("update follow-up = %q, want %q", got, "read-after-write")
	}
	if got := hooks.Semantics.DeleteFollowUp.Strategy; got != "confirm-delete" {
		t.Fatalf("delete follow-up = %q, want %q", got, "confirm-delete")
	}
	if len(hooks.Semantics.AuxiliaryOperations) != 0 {
		t.Fatalf("auxiliary operations = %#v, want scaffold placeholder removed", hooks.Semantics.AuxiliaryOperations)
	}
	if !containsString(hooks.Semantics.Mutation.Mutable, "parameters") {
		t.Fatalf("mutable fields = %#v, want parameters in reviewed mutable surface", hooks.Semantics.Mutation.Mutable)
	}
}

func TestBuildMediaWorkflowConfigurationCreateDetailsOmitsLockTimeCreated(t *testing.T) {
	t.Parallel()

	resource := newMediaWorkflowConfigurationTestResource()
	resource.Spec.Locks[0].TimeCreated = "2026-05-07T12:00:00Z"

	details, err := buildMediaWorkflowConfigurationCreateDetails(context.Background(), resource, "default")
	if err != nil {
		t.Fatalf("buildMediaWorkflowConfigurationCreateDetails() error = %v", err)
	}
	if len(details.Locks) != 1 {
		t.Fatalf("create locks = %d, want 1", len(details.Locks))
	}
	if details.Locks[0].TimeCreated != nil {
		t.Fatalf("create lock timeCreated = %#v, want omitted from reviewed request", details.Locks[0].TimeCreated)
	}
}

func TestMediaWorkflowConfigurationServiceClientUpdatesSupportedMutableDriftAndClearsEmptyValues(t *testing.T) {
	t.Parallel()

	original := newMediaWorkflowConfigurationTestResource()
	resource := newMediaWorkflowConfigurationTestResource()
	trackMediaWorkflowConfiguration(resource, testMediaWorkflowConfigurationID)
	resource.Spec.DisplayName = "configuration-beta"
	resource.Spec.Parameters = map[string]shared.JSONValue{}
	resource.Spec.FreeformTags = map[string]string{}
	resource.Spec.DefinedTags = map[string]shared.MapValue{}

	var updateRequest mediaservicessdk.UpdateMediaWorkflowConfigurationRequest

	client := newMediaWorkflowConfigurationTestClient(&fakeMediaWorkflowConfigurationOCIClient{
		getFn: func(_ context.Context, req mediaservicessdk.GetMediaWorkflowConfigurationRequest) (mediaservicessdk.GetMediaWorkflowConfigurationResponse, error) {
			if req.MediaWorkflowConfigurationId == nil || *req.MediaWorkflowConfigurationId != testMediaWorkflowConfigurationID {
				t.Fatalf("get mediaWorkflowConfigurationId = %v, want %q", req.MediaWorkflowConfigurationId, testMediaWorkflowConfigurationID)
			}
			return mediaservicessdk.GetMediaWorkflowConfigurationResponse{
				MediaWorkflowConfiguration: observedMediaWorkflowConfigurationFromSpec(
					testMediaWorkflowConfigurationID,
					original.Spec,
					mediaservicessdk.MediaWorkflowConfigurationLifecycleStateActive,
				),
			}, nil
		},
		updateFn: func(_ context.Context, req mediaservicessdk.UpdateMediaWorkflowConfigurationRequest) (mediaservicessdk.UpdateMediaWorkflowConfigurationResponse, error) {
			updateRequest = req
			return mediaservicessdk.UpdateMediaWorkflowConfigurationResponse{
				OpcRequestId: common.String("opc-update-1"),
				MediaWorkflowConfiguration: observedMediaWorkflowConfigurationFromSpec(
					testMediaWorkflowConfigurationID,
					resource.Spec,
					mediaservicessdk.MediaWorkflowConfigurationLifecycleStateActive,
				),
			}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful || response.ShouldRequeue {
		t.Fatalf("response = %#v, want successful update without requeue", response)
	}
	if updateRequest.MediaWorkflowConfigurationId == nil || *updateRequest.MediaWorkflowConfigurationId != testMediaWorkflowConfigurationID {
		t.Fatalf("update mediaWorkflowConfigurationId = %v, want %q", updateRequest.MediaWorkflowConfigurationId, testMediaWorkflowConfigurationID)
	}
	if updateRequest.IsLockOverride != nil {
		t.Fatalf("update isLockOverride = %#v, want reviewed hook field omission", updateRequest.IsLockOverride)
	}
	if updateRequest.UpdateMediaWorkflowConfigurationDetails.DisplayName == nil || *updateRequest.UpdateMediaWorkflowConfigurationDetails.DisplayName != "configuration-beta" {
		t.Fatalf("update displayName = %#v, want configuration-beta", updateRequest.UpdateMediaWorkflowConfigurationDetails.DisplayName)
	}
	if updateRequest.UpdateMediaWorkflowConfigurationDetails.Parameters == nil || len(updateRequest.UpdateMediaWorkflowConfigurationDetails.Parameters) != 0 {
		t.Fatalf("update parameters = %#v, want explicit empty map clear", updateRequest.UpdateMediaWorkflowConfigurationDetails.Parameters)
	}
	if updateRequest.UpdateMediaWorkflowConfigurationDetails.FreeformTags == nil || len(updateRequest.UpdateMediaWorkflowConfigurationDetails.FreeformTags) != 0 {
		t.Fatalf("update freeformTags = %#v, want explicit empty map clear", updateRequest.UpdateMediaWorkflowConfigurationDetails.FreeformTags)
	}
	if updateRequest.UpdateMediaWorkflowConfigurationDetails.DefinedTags == nil || len(updateRequest.UpdateMediaWorkflowConfigurationDetails.DefinedTags) != 0 {
		t.Fatalf("update definedTags = %#v, want explicit empty map clear", updateRequest.UpdateMediaWorkflowConfigurationDetails.DefinedTags)
	}

	body := mediaWorkflowConfigurationSerializedRequestBody(t, mediaservicessdk.UpdateMediaWorkflowConfigurationRequest{
		MediaWorkflowConfigurationId:            common.String(testMediaWorkflowConfigurationID),
		UpdateMediaWorkflowConfigurationDetails: updateRequest.UpdateMediaWorkflowConfigurationDetails,
	}, http.MethodPut, "/mediaWorkflowConfigurations/"+testMediaWorkflowConfigurationID)
	for _, want := range []string{
		`"displayName":"configuration-beta"`,
		`"parameters":{}`,
		`"freeformTags":{}`,
		`"definedTags":{}`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("update body %s does not contain %s", body, want)
		}
	}
	if resource.Status.OsokStatus.OpcRequestID != "opc-update-1" {
		t.Fatalf("status.opcRequestId = %q, want %q", resource.Status.OsokStatus.OpcRequestID, "opc-update-1")
	}
}

func TestNormalizeMediaWorkflowConfigurationDesiredStateClearsEquivalentLocks(t *testing.T) {
	t.Parallel()

	resource := newMediaWorkflowConfigurationTestResource()
	current := observedMediaWorkflowConfigurationFromSpec(
		testMediaWorkflowConfigurationID,
		resource.Spec,
		mediaservicessdk.MediaWorkflowConfigurationLifecycleStateActive,
	)
	now := common.SDKTime{Time: time.Date(2026, time.May, 7, 12, 34, 56, 0, time.UTC)}
	current.Locks[0].TimeCreated = &now

	normalizeMediaWorkflowConfigurationDesiredState(resource, current)
	if resource.Spec.Locks != nil {
		t.Fatalf("spec.locks = %#v, want normalized nil after equivalent OCI lock readback", resource.Spec.Locks)
	}
}

func TestValidateMediaWorkflowConfigurationCreateOnlyDriftRejectsLockDrift(t *testing.T) {
	t.Parallel()

	resource := newMediaWorkflowConfigurationTestResource()
	current := observedMediaWorkflowConfigurationFromSpec(
		testMediaWorkflowConfigurationID,
		resource.Spec,
		mediaservicessdk.MediaWorkflowConfigurationLifecycleStateActive,
	)
	current.Locks[0].Type = mediaservicessdk.ResourceLockTypeFull

	err := validateMediaWorkflowConfigurationCreateOnlyDrift(resource, current)
	if err == nil || !strings.Contains(err.Error(), "locks") {
		t.Fatalf("validateMediaWorkflowConfigurationCreateOnlyDrift() error = %v, want locks drift failure", err)
	}
}

func TestGuardMediaWorkflowConfigurationExistingBeforeCreateRequiresDisplayName(t *testing.T) {
	t.Parallel()

	resource := newMediaWorkflowConfigurationTestResource()
	resource.Spec.DisplayName = ""

	decision, err := guardMediaWorkflowConfigurationExistingBeforeCreate(context.Background(), resource)
	if err != nil {
		t.Fatalf("guardMediaWorkflowConfigurationExistingBeforeCreate() error = %v", err)
	}
	if decision != generatedruntime.ExistingBeforeCreateDecisionSkip {
		t.Fatalf("guardMediaWorkflowConfigurationExistingBeforeCreate() = %q, want %q", decision, generatedruntime.ExistingBeforeCreateDecisionSkip)
	}

	resource.Spec.DisplayName = "configuration-alpha"
	decision, err = guardMediaWorkflowConfigurationExistingBeforeCreate(context.Background(), resource)
	if err != nil {
		t.Fatalf("guardMediaWorkflowConfigurationExistingBeforeCreate() error = %v", err)
	}
	if decision != generatedruntime.ExistingBeforeCreateDecisionAllow {
		t.Fatalf("guardMediaWorkflowConfigurationExistingBeforeCreate() = %q, want %q", decision, generatedruntime.ExistingBeforeCreateDecisionAllow)
	}
}

func TestNewMediaWorkflowConfigurationServiceClientWithOCIClientReusesPagedDisplayNameMatch(t *testing.T) {
	t.Parallel()

	resource := newMediaWorkflowConfigurationTestResource()
	existingID := "ocid1.mediaworkflowconfiguration.oc1..existing"
	createCalled := false
	listCalls := 0
	var getRequest mediaservicessdk.GetMediaWorkflowConfigurationRequest

	client := newMediaWorkflowConfigurationTestClient(&fakeMediaWorkflowConfigurationOCIClient{
		createFn: func(context.Context, mediaservicessdk.CreateMediaWorkflowConfigurationRequest) (mediaservicessdk.CreateMediaWorkflowConfigurationResponse, error) {
			createCalled = true
			return mediaservicessdk.CreateMediaWorkflowConfigurationResponse{}, nil
		},
		listFn: func(_ context.Context, req mediaservicessdk.ListMediaWorkflowConfigurationsRequest) (mediaservicessdk.ListMediaWorkflowConfigurationsResponse, error) {
			listCalls++
			switch listCalls {
			case 1:
				if req.CompartmentId == nil || *req.CompartmentId != resource.Spec.CompartmentId {
					t.Fatalf("first ListMediaWorkflowConfigurationsRequest.CompartmentId = %#v, want %q", req.CompartmentId, resource.Spec.CompartmentId)
				}
				if req.DisplayName == nil || *req.DisplayName != resource.Spec.DisplayName {
					t.Fatalf("first ListMediaWorkflowConfigurationsRequest.DisplayName = %#v, want %q", req.DisplayName, resource.Spec.DisplayName)
				}
				return mediaservicessdk.ListMediaWorkflowConfigurationsResponse{
					MediaWorkflowConfigurationCollection: mediaservicessdk.MediaWorkflowConfigurationCollection{
						Items: []mediaservicessdk.MediaWorkflowConfigurationSummary{},
					},
					OpcNextPage: common.String("next-page"),
				}, nil
			case 2:
				if req.Page == nil || *req.Page != "next-page" {
					t.Fatalf("second ListMediaWorkflowConfigurationsRequest.Page = %#v, want next-page", req.Page)
				}
				return mediaservicessdk.ListMediaWorkflowConfigurationsResponse{
					MediaWorkflowConfigurationCollection: mediaservicessdk.MediaWorkflowConfigurationCollection{
						Items: []mediaservicessdk.MediaWorkflowConfigurationSummary{{
							Id:             common.String(existingID),
							CompartmentId:  common.String(resource.Spec.CompartmentId),
							DisplayName:    common.String(resource.Spec.DisplayName),
							LifecycleState: mediaservicessdk.MediaWorkflowConfigurationLifecycleStateActive,
						}},
					},
				}, nil
			default:
				t.Fatalf("unexpected ListMediaWorkflowConfigurations call #%d", listCalls)
				return mediaservicessdk.ListMediaWorkflowConfigurationsResponse{}, nil
			}
		},
		getFn: func(_ context.Context, req mediaservicessdk.GetMediaWorkflowConfigurationRequest) (mediaservicessdk.GetMediaWorkflowConfigurationResponse, error) {
			getRequest = req
			return mediaservicessdk.GetMediaWorkflowConfigurationResponse{
				MediaWorkflowConfiguration: observedMediaWorkflowConfigurationFromSpec(
					existingID,
					resource.Spec,
					mediaservicessdk.MediaWorkflowConfigurationLifecycleStateActive,
				),
			}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = false, want true")
	}
	if createCalled {
		t.Fatal("CreateOrUpdate() invoked CreateMediaWorkflowConfiguration, want existing-before-create reuse")
	}
	if getRequest.MediaWorkflowConfigurationId == nil || *getRequest.MediaWorkflowConfigurationId != existingID {
		t.Fatalf("GetMediaWorkflowConfigurationRequest.MediaWorkflowConfigurationId = %#v, want %q", getRequest.MediaWorkflowConfigurationId, existingID)
	}
	if resource.Status.Id != existingID {
		t.Fatalf("resource.Status.Id = %q, want %q", resource.Status.Id, existingID)
	}
}

func TestMediaWorkflowConfigurationServiceClientDeleteOmitsLockOverrideAndConfirmsDeleted(t *testing.T) {
	t.Parallel()

	resource := newMediaWorkflowConfigurationTestResource()
	trackMediaWorkflowConfiguration(resource, testMediaWorkflowConfigurationID)

	getCalls := 0
	var deleteRequest mediaservicessdk.DeleteMediaWorkflowConfigurationRequest

	client := newMediaWorkflowConfigurationTestClient(&fakeMediaWorkflowConfigurationOCIClient{
		getFn: func(_ context.Context, req mediaservicessdk.GetMediaWorkflowConfigurationRequest) (mediaservicessdk.GetMediaWorkflowConfigurationResponse, error) {
			getCalls++
			if req.MediaWorkflowConfigurationId == nil || *req.MediaWorkflowConfigurationId != testMediaWorkflowConfigurationID {
				t.Fatalf("get mediaWorkflowConfigurationId = %v, want %q", req.MediaWorkflowConfigurationId, testMediaWorkflowConfigurationID)
			}
			state := mediaservicessdk.MediaWorkflowConfigurationLifecycleStateActive
			if getCalls > 1 {
				state = mediaservicessdk.MediaWorkflowConfigurationLifecycleStateDeleted
			}
			return mediaservicessdk.GetMediaWorkflowConfigurationResponse{
				MediaWorkflowConfiguration: observedMediaWorkflowConfigurationFromSpec(
					testMediaWorkflowConfigurationID,
					resource.Spec,
					state,
				),
			}, nil
		},
		deleteFn: func(_ context.Context, req mediaservicessdk.DeleteMediaWorkflowConfigurationRequest) (mediaservicessdk.DeleteMediaWorkflowConfigurationResponse, error) {
			deleteRequest = req
			return mediaservicessdk.DeleteMediaWorkflowConfigurationResponse{
				OpcRequestId: common.String("opc-delete-1"),
			}, nil
		},
	})

	deleted, err := client.Delete(context.Background(), resource)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() deleted = false, want terminal delete confirmation")
	}
	if getCalls != 2 {
		t.Fatalf("GetMediaWorkflowConfiguration() calls = %d, want 2", getCalls)
	}
	if deleteRequest.MediaWorkflowConfigurationId == nil || *deleteRequest.MediaWorkflowConfigurationId != testMediaWorkflowConfigurationID {
		t.Fatalf("delete mediaWorkflowConfigurationId = %v, want %q", deleteRequest.MediaWorkflowConfigurationId, testMediaWorkflowConfigurationID)
	}
	if deleteRequest.IsLockOverride != nil {
		t.Fatalf("delete isLockOverride = %#v, want reviewed hook field omission", deleteRequest.IsLockOverride)
	}
	if resource.Status.LifecycleState != "DELETED" {
		t.Fatalf("status.lifecycleState = %q, want DELETED", resource.Status.LifecycleState)
	}
	if resource.Status.OsokStatus.DeletedAt == nil {
		t.Fatal("status.deletedAt = nil, want confirmed delete timestamp")
	}
	if resource.Status.OsokStatus.Reason != string(shared.Terminating) {
		t.Fatalf("status.reason = %q, want %q", resource.Status.OsokStatus.Reason, shared.Terminating)
	}
	if resource.Status.OsokStatus.OpcRequestID != "opc-delete-1" {
		t.Fatalf("status.opcRequestId = %q, want %q", resource.Status.OsokStatus.OpcRequestID, "opc-delete-1")
	}
}
