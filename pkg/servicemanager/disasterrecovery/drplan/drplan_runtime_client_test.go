/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package drplan

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	disasterrecoverysdk "github.com/oracle/oci-go-sdk/v65/disasterrecovery"
	disasterrecoveryv1beta1 "github.com/oracle/oci-service-operator/api/disasterrecovery/v1beta1"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type drPlanRequestBodyBuilder interface {
	HTTPRequest(
		method string,
		path string,
		binaryRequestBody *common.OCIReadSeekCloser,
		extraHeaders map[string]string,
	) (http.Request, error)
}

type fakeDrPlanOCIClient struct {
	getWorkRequestFn func(context.Context, disasterrecoverysdk.GetWorkRequestRequest) (disasterrecoverysdk.GetWorkRequestResponse, error)
}

func (f *fakeDrPlanOCIClient) CreateDrPlan(
	context.Context,
	disasterrecoverysdk.CreateDrPlanRequest,
) (disasterrecoverysdk.CreateDrPlanResponse, error) {
	return disasterrecoverysdk.CreateDrPlanResponse{}, nil
}

func (f *fakeDrPlanOCIClient) GetDrPlan(
	context.Context,
	disasterrecoverysdk.GetDrPlanRequest,
) (disasterrecoverysdk.GetDrPlanResponse, error) {
	return disasterrecoverysdk.GetDrPlanResponse{}, nil
}

func (f *fakeDrPlanOCIClient) ListDrPlans(
	context.Context,
	disasterrecoverysdk.ListDrPlansRequest,
) (disasterrecoverysdk.ListDrPlansResponse, error) {
	return disasterrecoverysdk.ListDrPlansResponse{}, nil
}

func (f *fakeDrPlanOCIClient) UpdateDrPlan(
	context.Context,
	disasterrecoverysdk.UpdateDrPlanRequest,
) (disasterrecoverysdk.UpdateDrPlanResponse, error) {
	return disasterrecoverysdk.UpdateDrPlanResponse{}, nil
}

func (f *fakeDrPlanOCIClient) DeleteDrPlan(
	context.Context,
	disasterrecoverysdk.DeleteDrPlanRequest,
) (disasterrecoverysdk.DeleteDrPlanResponse, error) {
	return disasterrecoverysdk.DeleteDrPlanResponse{}, nil
}

func (f *fakeDrPlanOCIClient) GetWorkRequest(
	ctx context.Context,
	request disasterrecoverysdk.GetWorkRequestRequest,
) (disasterrecoverysdk.GetWorkRequestResponse, error) {
	if f.getWorkRequestFn == nil {
		return disasterrecoverysdk.GetWorkRequestResponse{}, nil
	}
	return f.getWorkRequestFn(ctx, request)
}

func TestApplyDrPlanRuntimeHooksConfiguresReviewedSemantics(t *testing.T) {
	t.Parallel()

	client := &fakeDrPlanOCIClient{
		getWorkRequestFn: func(_ context.Context, request disasterrecoverysdk.GetWorkRequestRequest) (disasterrecoverysdk.GetWorkRequestResponse, error) {
			if got := drPlanStringValue(request.WorkRequestId); got != "wr-123" {
				t.Fatalf("GetWorkRequest id = %q, want wr-123", got)
			}
			return disasterrecoverysdk.GetWorkRequestResponse{
				WorkRequest: disasterrecoverysdk.WorkRequest{
					Id:            common.String("wr-123"),
					OperationType: disasterrecoverysdk.OperationTypeCreateDrPlan,
					Status:        disasterrecoverysdk.OperationStatusAccepted,
				},
			}, nil
		},
	}
	hooks := &DrPlanRuntimeHooks{}

	applyDrPlanRuntimeHooks(hooks, client, nil)

	if hooks.Semantics == nil {
		t.Fatal("hooks.Semantics = nil, want reviewed semantics")
	}
	if got := hooks.Semantics.CreateFollowUp.Strategy; got != "GetWorkRequest -> GetDrPlan" {
		t.Fatalf("CreateFollowUp.Strategy = %q, want GetWorkRequest -> GetDrPlan", got)
	}
	if got := hooks.Semantics.UpdateFollowUp.Strategy; got != "GetWorkRequest -> GetDrPlan" {
		t.Fatalf("UpdateFollowUp.Strategy = %q, want GetWorkRequest -> GetDrPlan", got)
	}
	if got := hooks.Semantics.DeleteFollowUp.Strategy; got != "GetDrPlan/ListDrPlans confirm-delete" {
		t.Fatalf("DeleteFollowUp.Strategy = %q, want GetDrPlan/ListDrPlans confirm-delete", got)
	}
	if got := hooks.Semantics.List.MatchFields; len(got) != 3 || got[0] != "drProtectionGroupId" || got[1] != "displayName" || got[2] != "type" {
		t.Fatalf("List.MatchFields = %#v, want drProtectionGroupId/displayName/type", got)
	}
	if got := hooks.Semantics.Mutation.ForceNew; len(got) != 3 || got[0] != "drProtectionGroupId" || got[1] != "sourcePlanId" || got[2] != "type" {
		t.Fatalf("Mutation.ForceNew = %#v, want drProtectionGroupId/sourcePlanId/type", got)
	}
	workRequest, err := hooks.Async.GetWorkRequest(context.Background(), "wr-123")
	if err != nil {
		t.Fatalf("Async.GetWorkRequest() error = %v", err)
	}
	current, ok := workRequest.(disasterrecoverysdk.WorkRequest)
	if !ok {
		t.Fatalf("Async.GetWorkRequest() type = %T, want disasterrecoverysdk.WorkRequest", workRequest)
	}
	if got := drPlanStringValue(current.Id); got != "wr-123" {
		t.Fatalf("workRequest.Id = %q, want wr-123", got)
	}
}

func TestBuildDrPlanCreateBodyOmitsPlanGroups(t *testing.T) {
	t.Parallel()

	resource := newDrPlanTestResource()
	resource.Spec.SourcePlanId = "ocid1.drplan.oc1..source"
	resource.Spec.FreeformTags = map[string]string{"managed-by": "osok"}
	resource.Spec.PlanGroups = []disasterrecoveryv1beta1.DrPlanPlanGroup{
		{
			Id:             "sgid1.group.oc1..pause",
			DisplayName:    "Pause Group",
			Type:           "USER_DEFINED_PAUSE",
			IsPauseEnabled: false,
		},
	}

	details, err := buildDrPlanCreateBody(resource)
	if err != nil {
		t.Fatalf("buildDrPlanCreateBody() error = %v", err)
	}

	body := drPlanSerializedRequestBody(t, disasterrecoverysdk.CreateDrPlanRequest{
		CreateDrPlanDetails: details,
	}, http.MethodPost, "/drPlans")
	for _, want := range []string{
		`"displayName":"drplan-sample"`,
		`"type":"SWITCHOVER"`,
		`"drProtectionGroupId":"ocid1.drprotectiongroup.oc1..primary"`,
		`"sourcePlanId":"ocid1.drplan.oc1..source"`,
		`"managed-by":"osok"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("request body %s does not contain %s", body, want)
		}
	}
	if strings.Contains(body, `"planGroups"`) {
		t.Fatalf("request body %s unexpectedly contains planGroups", body)
	}
}

func TestBuildDrPlanUpdateBodyPreservesExplicitFalseBooleans(t *testing.T) {
	t.Parallel()

	resource := newDrPlanTestResource()
	resource.Spec.PlanGroups = []disasterrecoveryv1beta1.DrPlanPlanGroup{
		{
			Id:             "sgid1.group.oc1..pause",
			DisplayName:    "Pause Group",
			Type:           "USER_DEFINED_PAUSE",
			IsPauseEnabled: false,
			Steps: []disasterrecoveryv1beta1.DrPlanPlanGroupStep{
				{
					Id:          "sgid1.step.oc1..pause",
					DisplayName: "Pause Step",
					ErrorMode:   "STOP_ON_ERROR",
					Timeout:     600,
					IsEnabled:   false,
					UserDefinedStep: disasterrecoveryv1beta1.DrPlanPlanGroupStepUserDefinedStep{
						StepType:        "RUN_OBJECTSTORE_SCRIPT",
						RunOnInstanceId: "ocid1.instance.oc1..runner",
						ScriptCommand:   "/bin/bash /tmp/precheck.sh",
						RunAsUser:       "opc",
						ObjectStorageScriptLocation: disasterrecoveryv1beta1.DrPlanPlanGroupStepUserDefinedStepObjectStorageScriptLocation{
							Namespace: "tenant-a",
							Bucket:    "dr-scripts",
							Object:    "precheck.sh",
						},
					},
				},
			},
		},
	}
	current := disasterrecoverysdk.GetDrPlanResponse{
		DrPlan: disasterrecoverysdk.DrPlan{
			Id:                      common.String("ocid1.drplan.oc1..current"),
			DisplayName:             common.String(resource.Spec.DisplayName),
			CompartmentId:           common.String("ocid1.compartment.oc1..example"),
			Type:                    disasterrecoverysdk.DrPlanTypeSwitchover,
			TimeCreated:             sdkTimePtr(time.Date(2026, time.May, 12, 10, 0, 0, 0, time.UTC)),
			TimeUpdated:             sdkTimePtr(time.Date(2026, time.May, 12, 10, 5, 0, 0, time.UTC)),
			DrProtectionGroupId:     common.String(resource.Spec.DrProtectionGroupId),
			PeerDrProtectionGroupId: common.String("ocid1.drprotectiongroup.oc1..peer"),
			PeerRegion:              common.String("us-ashburn-1"),
			LifecycleState:          disasterrecoverysdk.DrPlanLifecycleStateActive,
		},
	}

	details, updateNeeded, err := buildDrPlanUpdateBody(resource, current)
	if err != nil {
		t.Fatalf("buildDrPlanUpdateBody() error = %v", err)
	}
	if !updateNeeded {
		t.Fatal("buildDrPlanUpdateBody() updateNeeded = false, want true")
	}

	body := drPlanSerializedRequestBody(t, disasterrecoverysdk.UpdateDrPlanRequest{
		DrPlanId:            common.String("ocid1.drplan.oc1..current"),
		UpdateDrPlanDetails: details,
	}, http.MethodPut, "/drPlans/ocid1.drplan.oc1..current")
	for _, want := range []string{
		`"isPauseEnabled":false`,
		`"isEnabled":false`,
		`"stepType":"RUN_OBJECTSTORE_SCRIPT"`,
		`"namespace":"tenant-a"`,
		`"bucket":"dr-scripts"`,
		`"object":"precheck.sh"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("request body %s does not contain %s", body, want)
		}
	}
	for _, unwanted := range []string{
		`"sourcePlanId"`,
		`"drProtectionGroupId"`,
	} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("request body %s unexpectedly contains %s", body, unwanted)
		}
	}
}

func TestBuildDrPlanUpdateBodyIgnoresServiceOwnedResponseFields(t *testing.T) {
	t.Parallel()

	resource := newDrPlanTestResource()
	resource.Spec.PlanGroups = []disasterrecoveryv1beta1.DrPlanPlanGroup{
		{
			Id:             "sgid1.group.oc1..pause",
			DisplayName:    "Pause Group",
			Type:           "USER_DEFINED_PAUSE",
			IsPauseEnabled: false,
			Steps: []disasterrecoveryv1beta1.DrPlanPlanGroupStep{
				{
					Id:          "sgid1.step.oc1..pause",
					DisplayName: "Pause Step",
					ErrorMode:   "STOP_ON_ERROR",
					Timeout:     600,
					IsEnabled:   false,
					UserDefinedStep: disasterrecoveryv1beta1.DrPlanPlanGroupStepUserDefinedStep{
						StepType:        "RUN_OBJECTSTORE_SCRIPT",
						RunOnInstanceId: "ocid1.instance.oc1..runner",
						ScriptCommand:   "/bin/bash /tmp/precheck.sh",
						RunAsUser:       "opc",
						ObjectStorageScriptLocation: disasterrecoveryv1beta1.DrPlanPlanGroupStepUserDefinedStepObjectStorageScriptLocation{
							Namespace: "tenant-a",
							Bucket:    "dr-scripts",
							Object:    "precheck.sh",
						},
					},
				},
			},
		},
	}
	current := disasterrecoverysdk.GetDrPlanResponse{
		DrPlan: disasterrecoverysdk.DrPlan{
			Id:                      common.String("ocid1.drplan.oc1..current"),
			DisplayName:             common.String(resource.Spec.DisplayName),
			CompartmentId:           common.String("ocid1.compartment.oc1..example"),
			Type:                    disasterrecoverysdk.DrPlanTypeSwitchover,
			TimeCreated:             sdkTimePtr(time.Date(2026, time.May, 12, 10, 0, 0, 0, time.UTC)),
			TimeUpdated:             sdkTimePtr(time.Date(2026, time.May, 12, 10, 5, 0, 0, time.UTC)),
			DrProtectionGroupId:     common.String(resource.Spec.DrProtectionGroupId),
			PeerDrProtectionGroupId: common.String("ocid1.drprotectiongroup.oc1..peer"),
			PeerRegion:              common.String("us-ashburn-1"),
			LifecycleState:          disasterrecoverysdk.DrPlanLifecycleStateActive,
			PlanGroups: []disasterrecoverysdk.DrPlanGroup{
				{
					Id:             common.String("sgid1.group.oc1..pause"),
					Type:           disasterrecoverysdk.DrPlanGroupTypeUserDefinedPause,
					DisplayName:    common.String("Pause Group"),
					IsPauseEnabled: common.Bool(false),
					Steps: []disasterrecoverysdk.DrPlanStep{
						{
							Id:              common.String("sgid1.step.oc1..pause"),
							GroupId:         common.String("sgid1.group.oc1..pause"),
							Type:            disasterrecoverysdk.DrPlanStepTypeUserDefined,
							DisplayName:     common.String("Pause Step"),
							TypeDisplayName: common.String("User Defined"),
							ErrorMode:       disasterrecoverysdk.DrPlanStepErrorModeStopOnError,
							Timeout:         common.Int(600),
							IsEnabled:       common.Bool(false),
							UserDefinedStep: disasterrecoverysdk.RunObjectStoreScriptUserDefinedStep{
								RunOnInstanceId:     common.String("ocid1.instance.oc1..runner"),
								RunOnInstanceRegion: common.String("us-phoenix-1"),
								ObjectStorageScriptLocation: &disasterrecoverysdk.ObjectStorageScriptLocation{
									Namespace: common.String("tenant-a"),
									Bucket:    common.String("dr-scripts"),
									Object:    common.String("precheck.sh"),
								},
								ScriptCommand: common.String("/bin/bash /tmp/precheck.sh"),
								RunAsUser:     common.String("opc"),
							},
						},
					},
				},
			},
		},
	}

	_, updateNeeded, err := buildDrPlanUpdateBody(resource, current)
	if err != nil {
		t.Fatalf("buildDrPlanUpdateBody() error = %v", err)
	}
	if updateNeeded {
		t.Fatal("buildDrPlanUpdateBody() updateNeeded = true, want false when only service-owned fields differ")
	}
}

func TestBuildDrPlanUpdateBodySkipsPlanGroupsWhenSpecOmitsThem(t *testing.T) {
	t.Parallel()

	resource := newDrPlanTestResource()
	current := disasterrecoverysdk.GetDrPlanResponse{
		DrPlan: disasterrecoverysdk.DrPlan{
			Id:                      common.String("ocid1.drplan.oc1..current"),
			DisplayName:             common.String(resource.Spec.DisplayName),
			CompartmentId:           common.String("ocid1.compartment.oc1..example"),
			Type:                    disasterrecoverysdk.DrPlanTypeSwitchover,
			TimeCreated:             sdkTimePtr(time.Date(2026, time.May, 12, 10, 0, 0, 0, time.UTC)),
			TimeUpdated:             sdkTimePtr(time.Date(2026, time.May, 12, 10, 5, 0, 0, time.UTC)),
			DrProtectionGroupId:     common.String(resource.Spec.DrProtectionGroupId),
			PeerDrProtectionGroupId: common.String("ocid1.drprotectiongroup.oc1..peer"),
			PeerRegion:              common.String("us-ashburn-1"),
			LifecycleState:          disasterrecoverysdk.DrPlanLifecycleStateActive,
			PlanGroups: []disasterrecoverysdk.DrPlanGroup{
				{
					Id:          common.String("sgid1.group.oc1..existing"),
					Type:        disasterrecoverysdk.DrPlanGroupTypeBuiltIn,
					DisplayName: common.String("Built In Group"),
				},
			},
		},
	}

	_, updateNeeded, err := buildDrPlanUpdateBody(resource, current)
	if err != nil {
		t.Fatalf("buildDrPlanUpdateBody() error = %v", err)
	}
	if updateNeeded {
		t.Fatal("buildDrPlanUpdateBody() updateNeeded = true, want false when spec.planGroups is omitted")
	}
}

func TestRecoverDrPlanIDFromGeneratedWorkRequestPrefersDrPlanResource(t *testing.T) {
	t.Parallel()

	workRequest := disasterrecoverysdk.WorkRequest{
		Id:            common.String("wr-123"),
		OperationType: disasterrecoverysdk.OperationTypeCreateDrPlan,
		Status:        disasterrecoverysdk.OperationStatusSucceeded,
		Resources: []disasterrecoverysdk.WorkRequestResource{
			{
				EntityType: common.String("DrProtectionGroup"),
				ActionType: disasterrecoverysdk.ActionTypeCreated,
				Identifier: common.String("ocid1.drprotectiongroup.oc1..primary"),
			},
			{
				EntityType: common.String("DrPlan"),
				ActionType: disasterrecoverysdk.ActionTypeCreated,
				Identifier: common.String("ocid1.drplan.oc1..created"),
			},
		},
	}

	resourceID, err := recoverDrPlanIDFromGeneratedWorkRequest(nil, workRequest, shared.OSOKAsyncPhaseCreate)
	if err != nil {
		t.Fatalf("recoverDrPlanIDFromGeneratedWorkRequest() error = %v", err)
	}
	if resourceID != "ocid1.drplan.oc1..created" {
		t.Fatalf("recoverDrPlanIDFromGeneratedWorkRequest() = %q, want ocid1.drplan.oc1..created", resourceID)
	}
}

func newDrPlanTestResource() *disasterrecoveryv1beta1.DrPlan {
	return &disasterrecoveryv1beta1.DrPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "drplan-sample",
			Namespace: "default",
		},
		Spec: disasterrecoveryv1beta1.DrPlanSpec{
			DisplayName:         "drplan-sample",
			Type:                "SWITCHOVER",
			DrProtectionGroupId: "ocid1.drprotectiongroup.oc1..primary",
		},
	}
}

func sdkTimePtr(t time.Time) *common.SDKTime {
	value := common.SDKTime{Time: t}
	return &value
}

func drPlanSerializedRequestBody(
	t *testing.T,
	request drPlanRequestBodyBuilder,
	method string,
	path string,
) string {
	t.Helper()

	httpRequest, err := request.HTTPRequest(method, path, nil, nil)
	if err != nil {
		t.Fatalf("HTTPRequest() error = %v", err)
	}
	if httpRequest.Body == nil {
		return ""
	}

	body, err := io.ReadAll(httpRequest.Body)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	return string(body)
}
