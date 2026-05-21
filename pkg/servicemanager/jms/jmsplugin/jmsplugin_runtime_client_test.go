package jmsplugin

import (
	"context"
	"maps"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	jmssdk "github.com/oracle/oci-go-sdk/v65/jms"
	jmsv1beta1 "github.com/oracle/oci-service-operator/api/jms/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/errorutil/errortest"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type fakeJmsPluginOCIClient struct {
	createFn func(context.Context, jmssdk.CreateJmsPluginRequest) (jmssdk.CreateJmsPluginResponse, error)
	getFn    func(context.Context, jmssdk.GetJmsPluginRequest) (jmssdk.GetJmsPluginResponse, error)
	listFn   func(context.Context, jmssdk.ListJmsPluginsRequest) (jmssdk.ListJmsPluginsResponse, error)
	updateFn func(context.Context, jmssdk.UpdateJmsPluginRequest) (jmssdk.UpdateJmsPluginResponse, error)
	deleteFn func(context.Context, jmssdk.DeleteJmsPluginRequest) (jmssdk.DeleteJmsPluginResponse, error)
}

func (f *fakeJmsPluginOCIClient) CreateJmsPlugin(
	ctx context.Context,
	req jmssdk.CreateJmsPluginRequest,
) (jmssdk.CreateJmsPluginResponse, error) {
	if f.createFn != nil {
		return f.createFn(ctx, req)
	}
	return jmssdk.CreateJmsPluginResponse{}, nil
}

func (f *fakeJmsPluginOCIClient) GetJmsPlugin(
	ctx context.Context,
	req jmssdk.GetJmsPluginRequest,
) (jmssdk.GetJmsPluginResponse, error) {
	if f.getFn != nil {
		return f.getFn(ctx, req)
	}
	return jmssdk.GetJmsPluginResponse{}, errortest.NewServiceError(404, "NotFound", "missing JmsPlugin")
}

func (f *fakeJmsPluginOCIClient) ListJmsPlugins(
	ctx context.Context,
	req jmssdk.ListJmsPluginsRequest,
) (jmssdk.ListJmsPluginsResponse, error) {
	if f.listFn != nil {
		return f.listFn(ctx, req)
	}
	return jmssdk.ListJmsPluginsResponse{}, nil
}

func (f *fakeJmsPluginOCIClient) UpdateJmsPlugin(
	ctx context.Context,
	req jmssdk.UpdateJmsPluginRequest,
) (jmssdk.UpdateJmsPluginResponse, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, req)
	}
	return jmssdk.UpdateJmsPluginResponse{}, nil
}

func (f *fakeJmsPluginOCIClient) DeleteJmsPlugin(
	ctx context.Context,
	req jmssdk.DeleteJmsPluginRequest,
) (jmssdk.DeleteJmsPluginResponse, error) {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, req)
	}
	return jmssdk.DeleteJmsPluginResponse{}, nil
}

func TestReviewedJmsPluginRuntimeSemantics(t *testing.T) {
	t.Parallel()

	got := reviewedJmsPluginRuntimeSemantics()
	if got == nil {
		t.Fatal("reviewedJmsPluginRuntimeSemantics() = nil")
	}

	if got.FormalService != "jms" {
		t.Fatalf("FormalService = %q, want jms", got.FormalService)
	}
	if got.FormalSlug != "jmsplugin" {
		t.Fatalf("FormalSlug = %q, want jmsplugin", got.FormalSlug)
	}
	if got.Async == nil || got.Async.Strategy != "none" || got.Async.Runtime != "generatedruntime" {
		t.Fatalf("Async = %#v, want generatedruntime none semantics", got.Async)
	}
	assertJmsPluginStringSliceEqual(t, "Lifecycle.ActiveStates", got.Lifecycle.ActiveStates, []string{"ACTIVE", "INACTIVE"})
	assertJmsPluginStringSliceEqual(t, "Delete.TerminalStates", got.Delete.TerminalStates, []string{"DELETED"})
	assertJmsPluginStringSliceEqual(t, "List.MatchFields", got.List.MatchFields, []string{"compartmentId", "agentId"})
	assertJmsPluginStringSliceEqual(t, "Mutation.Mutable", got.Mutation.Mutable, []string{"definedTags", "fleetId", "freeformTags"})
	assertJmsPluginStringSliceEqual(t, "Mutation.ForceNew", got.Mutation.ForceNew, []string{"agentId", "agentType", "compartmentId"})
	if got.CreateFollowUp.Strategy != "read-after-write" {
		t.Fatalf("CreateFollowUp.Strategy = %q, want read-after-write", got.CreateFollowUp.Strategy)
	}
	if got.UpdateFollowUp.Strategy != "read-after-write" {
		t.Fatalf("UpdateFollowUp.Strategy = %q, want read-after-write", got.UpdateFollowUp.Strategy)
	}
	if got.DeleteFollowUp.Strategy != "confirm-delete" {
		t.Fatalf("DeleteFollowUp.Strategy = %q, want confirm-delete", got.DeleteFollowUp.Strategy)
	}
}

func TestGuardJmsPluginExistingBeforeCreate(t *testing.T) {
	t.Parallel()

	resource := makeJmsPluginResource()
	resource.Spec.AgentId = ""

	decision, err := guardJmsPluginExistingBeforeCreate(context.Background(), resource)
	if err == nil || !strings.Contains(err.Error(), "spec.agentId") {
		t.Fatalf("guardJmsPluginExistingBeforeCreate(missing agentId) error = %v, want spec.agentId validation", err)
	}
	if decision != generatedruntime.ExistingBeforeCreateDecisionFail {
		t.Fatalf("guardJmsPluginExistingBeforeCreate(missing agentId) = %q, want fail", decision)
	}

	resource.Spec.AgentId = "ocid1.managementagent.oc1..agent"
	decision, err = guardJmsPluginExistingBeforeCreate(context.Background(), resource)
	if err != nil {
		t.Fatalf("guardJmsPluginExistingBeforeCreate(complete resource) error = %v", err)
	}
	if decision != generatedruntime.ExistingBeforeCreateDecisionAllow {
		t.Fatalf("guardJmsPluginExistingBeforeCreate(complete resource) = %q, want allow", decision)
	}
}

func TestBuildJmsPluginUpdateBodySupportsFleetMoveAndTagClears(t *testing.T) {
	t.Parallel()

	currentResource := makeJmsPluginResource()
	currentResource.Spec.FleetId = "ocid1.fleet.oc1..current"
	currentResource.Spec.FreeformTags = map[string]string{"owner": "platform"}
	currentResource.Spec.DefinedTags = map[string]shared.MapValue{
		"Operations": {"team": "platform"},
	}

	desired := makeJmsPluginResource()
	desired.Spec.FleetId = "ocid1.fleet.oc1..desired"
	desired.Spec.FreeformTags = map[string]string{}
	desired.Spec.DefinedTags = map[string]shared.MapValue{}

	body, updateNeeded, err := buildJmsPluginUpdateBody(
		desired,
		jmssdk.GetJmsPluginResponse{
			JmsPlugin: makeSDKJmsPlugin("ocid1.jmsplugin.oc1..existing", currentResource, jmssdk.JmsPluginLifecycleStateActive),
		},
	)
	if err != nil {
		t.Fatalf("buildJmsPluginUpdateBody() error = %v", err)
	}
	if !updateNeeded {
		t.Fatal("buildJmsPluginUpdateBody() updateNeeded = false, want true")
	}
	requireJmsPluginStringPtr(t, "details.fleetId", body.FleetId, desired.Spec.FleetId)
	if len(body.FreeformTags) != 0 {
		t.Fatalf("details.FreeformTags = %#v, want empty map for clear", body.FreeformTags)
	}
	if len(body.DefinedTags) != 0 {
		t.Fatalf("details.DefinedTags = %#v, want empty map for clear", body.DefinedTags)
	}
}

func TestJmsPluginCreateOrUpdateTreatsInactiveAsSuccess(t *testing.T) {
	t.Parallel()

	resource := makeJmsPluginResource()
	resource.Spec.FleetId = ""
	createdID := "ocid1.jmsplugin.oc1..created"
	createCalls := 0
	getCalls := 0

	client := newTestJmsPluginClient(&fakeJmsPluginOCIClient{
		createFn: func(_ context.Context, req jmssdk.CreateJmsPluginRequest) (jmssdk.CreateJmsPluginResponse, error) {
			createCalls++
			requireJmsPluginStringPtr(t, "create agentId", req.CreateJmsPluginDetails.AgentId, resource.Spec.AgentId)
			requireJmsPluginStringPtr(t, "create compartmentId", req.CreateJmsPluginDetails.CompartmentId, resource.Spec.CompartmentId)
			if req.CreateJmsPluginDetails.FleetId != nil {
				t.Fatalf("create fleetId = %v, want nil for fleet-less registration", *req.CreateJmsPluginDetails.FleetId)
			}
			if !jmsPluginJSONEqual(req.CreateJmsPluginDetails.DefinedTags, jmsPluginDefinedTagsFromSpec(resource.Spec.DefinedTags)) {
				t.Fatalf("create definedTags = %#v, want %#v", req.CreateJmsPluginDetails.DefinedTags, jmsPluginDefinedTagsFromSpec(resource.Spec.DefinedTags))
			}
			return jmssdk.CreateJmsPluginResponse{
				JmsPlugin: makeSDKJmsPlugin(createdID, resource, jmssdk.JmsPluginLifecycleStateInactive),
			}, nil
		},
		getFn: func(_ context.Context, req jmssdk.GetJmsPluginRequest) (jmssdk.GetJmsPluginResponse, error) {
			getCalls++
			requireJmsPluginStringPtr(t, "get jmsPluginId", req.JmsPluginId, createdID)
			return jmssdk.GetJmsPluginResponse{
				JmsPlugin: makeSDKJmsPlugin(createdID, resource, jmssdk.JmsPluginLifecycleStateInactive),
			}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = false, want true for INACTIVE steady state")
	}
	if response.ShouldRequeue {
		t.Fatal("CreateOrUpdate() ShouldRequeue = true, want false")
	}
	if createCalls != 1 {
		t.Fatalf("create calls = %d, want 1", createCalls)
	}
	if getCalls != 1 {
		t.Fatalf("get calls = %d, want 1", getCalls)
	}
	if resource.Status.Id != createdID {
		t.Fatalf("status.id = %q, want %q", resource.Status.Id, createdID)
	}
	if resource.Status.LifecycleState != string(jmssdk.JmsPluginLifecycleStateInactive) {
		t.Fatalf("status.lifecycleState = %q, want INACTIVE", resource.Status.LifecycleState)
	}
}

func TestJmsPluginCreateOrUpdateReusesPaginatedAgentMatchAndUpdatesFleet(t *testing.T) {
	t.Parallel()

	resource := makeJmsPluginResource()
	resource.Spec.FleetId = "ocid1.fleet.oc1..desired"
	existing := makeJmsPluginResource()
	existing.Spec.FleetId = "ocid1.fleet.oc1..current"
	existingID := "ocid1.jmsplugin.oc1..existing"
	listCalls := 0
	getCalls := 0
	updateCalls := 0
	createCalls := 0

	client := newTestJmsPluginClient(&fakeJmsPluginOCIClient{
		listFn: func(_ context.Context, req jmssdk.ListJmsPluginsRequest) (jmssdk.ListJmsPluginsResponse, error) {
			listCalls++
			requireJmsPluginStringPtr(t, "list compartmentId", req.CompartmentId, resource.Spec.CompartmentId)
			requireJmsPluginStringPtr(t, "list agentId", req.AgentId, resource.Spec.AgentId)
			if req.FleetId != nil {
				t.Fatalf("list fleetId = %v, want nil so fleet moves can reuse existing plugins", *req.FleetId)
			}
			switch listCalls {
			case 1:
				if req.Page != nil {
					t.Fatalf("list page = %v, want nil on first page", *req.Page)
				}
				return jmssdk.ListJmsPluginsResponse{
					JmsPluginCollection: jmssdk.JmsPluginCollection{Items: []jmssdk.JmsPluginSummary{}},
					OpcNextPage:         common.String("page2"),
				}, nil
			case 2:
				requireJmsPluginStringPtr(t, "list page", req.Page, "page2")
				return jmssdk.ListJmsPluginsResponse{
					JmsPluginCollection: jmssdk.JmsPluginCollection{
						Items: []jmssdk.JmsPluginSummary{
							makeSDKJmsPluginSummary(existingID, existing, jmssdk.JmsPluginLifecycleStateActive),
						},
					},
				}, nil
			default:
				t.Fatalf("unexpected list call %d", listCalls)
				return jmssdk.ListJmsPluginsResponse{}, nil
			}
		},
		getFn: func(_ context.Context, req jmssdk.GetJmsPluginRequest) (jmssdk.GetJmsPluginResponse, error) {
			getCalls++
			requireJmsPluginStringPtr(t, "get jmsPluginId", req.JmsPluginId, existingID)
			switch getCalls {
			case 1:
				return jmssdk.GetJmsPluginResponse{
					JmsPlugin: makeSDKJmsPlugin(existingID, existing, jmssdk.JmsPluginLifecycleStateActive),
				}, nil
			case 2:
				return jmssdk.GetJmsPluginResponse{
					JmsPlugin: makeSDKJmsPlugin(existingID, resource, jmssdk.JmsPluginLifecycleStateActive),
				}, nil
			default:
				t.Fatalf("unexpected get call %d", getCalls)
				return jmssdk.GetJmsPluginResponse{}, nil
			}
		},
		updateFn: func(_ context.Context, req jmssdk.UpdateJmsPluginRequest) (jmssdk.UpdateJmsPluginResponse, error) {
			updateCalls++
			requireJmsPluginStringPtr(t, "update jmsPluginId", req.JmsPluginId, existingID)
			requireJmsPluginStringPtr(t, "update fleetId", req.UpdateJmsPluginDetails.FleetId, resource.Spec.FleetId)
			return jmssdk.UpdateJmsPluginResponse{
				JmsPlugin: makeSDKJmsPlugin(existingID, resource, jmssdk.JmsPluginLifecycleStateActive),
			}, nil
		},
		createFn: func(_ context.Context, _ jmssdk.CreateJmsPluginRequest) (jmssdk.CreateJmsPluginResponse, error) {
			createCalls++
			return jmssdk.CreateJmsPluginResponse{}, nil
		},
	})

	response, err := client.CreateOrUpdate(context.Background(), resource, ctrl.Request{})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if !response.IsSuccessful {
		t.Fatal("CreateOrUpdate() IsSuccessful = false, want true")
	}
	if response.ShouldRequeue {
		t.Fatal("CreateOrUpdate() ShouldRequeue = true, want false")
	}
	if createCalls != 0 {
		t.Fatalf("create calls = %d, want 0 when paginated list reuse succeeds", createCalls)
	}
	if listCalls != 2 {
		t.Fatalf("list calls = %d, want 2 for pagination", listCalls)
	}
	if getCalls != 2 {
		t.Fatalf("get calls = %d, want 2 (live read + follow-up)", getCalls)
	}
	if updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", updateCalls)
	}
	if resource.Status.Id != existingID {
		t.Fatalf("status.id = %q, want %q", resource.Status.Id, existingID)
	}
	if resource.Status.FleetId != resource.Spec.FleetId {
		t.Fatalf("status.fleetId = %q, want %q", resource.Status.FleetId, resource.Spec.FleetId)
	}
}

func newTestJmsPluginClient(client *fakeJmsPluginOCIClient) JmsPluginServiceClient {
	return newJmsPluginServiceClientWithOCIClient(loggerutil.OSOKLogger{}, client)
}

func makeJmsPluginResource() *jmsv1beta1.JmsPlugin {
	return &jmsv1beta1.JmsPlugin{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-jmsplugin",
			Namespace: "default",
		},
		Spec: jmsv1beta1.JmsPluginSpec{
			AgentId:       "ocid1.managementagent.oc1..agent",
			CompartmentId: "ocid1.compartment.oc1..jms",
			AgentType:     string(jmssdk.AgentTypeOca),
			FleetId:       "",
			FreeformTags: map[string]string{
				"env": "dev",
			},
			DefinedTags: map[string]shared.MapValue{
				"Operations": {"team": "platform"},
			},
		},
	}
}

func makeSDKJmsPlugin(
	id string,
	resource *jmsv1beta1.JmsPlugin,
	lifecycle jmssdk.JmsPluginLifecycleStateEnum,
) jmssdk.JmsPlugin {
	if resource == nil {
		resource = makeJmsPluginResource()
	}
	registered := common.SDKTime{Time: time.Unix(1713240000, 0).UTC()}
	lastSeen := common.SDKTime{Time: time.Unix(1713243600, 0).UTC()}
	availability := jmssdk.JmsPluginAvailabilityStatusActive
	if lifecycle == jmssdk.JmsPluginLifecycleStateDeleted {
		availability = jmssdk.JmsPluginAvailabilityStatusNotAvailable
	}

	var fleetID *string
	if strings.TrimSpace(resource.Spec.FleetId) != "" {
		fleetID = common.String(strings.TrimSpace(resource.Spec.FleetId))
	}

	agentType := jmssdk.AgentTypeEnum(strings.TrimSpace(resource.Spec.AgentType))
	if agentType == "" {
		agentType = jmssdk.AgentTypeOca
	}

	return jmssdk.JmsPlugin{
		Id:                 common.String(id),
		AgentId:            common.String(resource.Spec.AgentId),
		AgentType:          agentType,
		LifecycleState:     lifecycle,
		AvailabilityStatus: availability,
		TimeRegistered:     &registered,
		FleetId:            fleetID,
		CompartmentId:      common.String(resource.Spec.CompartmentId),
		Hostname:           common.String("agent.example.internal"),
		PluginVersion:      common.String("1.2.3"),
		TimeLastSeen:       &lastSeen,
		DefinedTags:        jmsPluginDefinedTagsFromSpec(resource.Spec.DefinedTags),
		FreeformTags:       maps.Clone(resource.Spec.FreeformTags),
		SystemTags: map[string]map[string]interface{}{
			"orcl-cloud": {"createdBy": "unit-test"},
		},
	}
}

func makeSDKJmsPluginSummary(
	id string,
	resource *jmsv1beta1.JmsPlugin,
	lifecycle jmssdk.JmsPluginLifecycleStateEnum,
) jmssdk.JmsPluginSummary {
	current := makeSDKJmsPlugin(id, resource, lifecycle)
	return jmssdk.JmsPluginSummary{
		Id:                 current.Id,
		AgentId:            current.AgentId,
		AgentType:          current.AgentType,
		LifecycleState:     current.LifecycleState,
		AvailabilityStatus: current.AvailabilityStatus,
		TimeRegistered:     current.TimeRegistered,
		FleetId:            current.FleetId,
		CompartmentId:      current.CompartmentId,
		Hostname:           current.Hostname,
		OsFamily:           current.OsFamily,
		OsArchitecture:     current.OsArchitecture,
		OsDistribution:     current.OsDistribution,
		PluginVersion:      current.PluginVersion,
		TimeLastSeen:       current.TimeLastSeen,
		DefinedTags:        current.DefinedTags,
		FreeformTags:       current.FreeformTags,
		SystemTags:         current.SystemTags,
	}
}

func assertJmsPluginStringSliceEqual(t *testing.T, label string, got []string, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}

func requireJmsPluginStringPtr(t *testing.T, label string, value *string, want string) {
	t.Helper()
	if value == nil {
		t.Fatalf("%s = nil, want %q", label, want)
	}
	if got := strings.TrimSpace(*value); got != want {
		t.Fatalf("%s = %q, want %q", label, got, want)
	}
}
