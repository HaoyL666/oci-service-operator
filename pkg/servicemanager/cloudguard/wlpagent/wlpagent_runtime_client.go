/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package wlpagent

import (
	"context"

	cloudguardv1beta1 "github.com/oracle/oci-service-operator/api/cloudguard/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/loggerutil"
	"github.com/oracle/oci-service-operator/pkg/servicemanager"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
	shared "github.com/oracle/oci-service-operator/pkg/shared"
	"github.com/oracle/oci-service-operator/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func init() {
	registerWlpAgentRuntimeHooksMutator(func(_ *WlpAgentServiceManager, hooks *WlpAgentRuntimeHooks) {
		hooks.Semantics = newWlpAgentRuntimeSemantics()
		hooks.WrapGeneratedClient = append(hooks.WrapGeneratedClient, wrapWlpAgentStateFreeClient)
	})
}

func newWlpAgentRuntimeSemantics() *generatedruntime.Semantics {
	return &generatedruntime.Semantics{
		FormalService:     "cloudguard",
		FormalSlug:        "wlpagent",
		StatusProjection:  "required",
		SecretSideEffects: "none",
		FinalizerPolicy:   "retain-until-confirmed-delete",
		Async: &generatedruntime.AsyncSemantics{
			Strategy:             "none",
			Runtime:              "generatedruntime",
			FormalClassification: "none",
		},
		Delete: generatedruntime.DeleteSemantics{Policy: "required", TerminalStates: []string{"DELETED"}},
		List: &generatedruntime.ListSemantics{
			ResponseItemsField: "Items",
			MatchFields:        []string{"compartmentId", "agentVersion"},
		},
		Mutation: generatedruntime.MutationSemantics{
			Mutable:       []string{"certificateSignedRequest", "freeformTags", "definedTags"},
			ForceNew:      []string{"compartmentId", "agentVersion", "osInfo"},
			ConflictsWith: map[string][]string{},
		},
		CreateFollowUp: generatedruntime.FollowUpSemantics{Strategy: "read-after-write"},
		UpdateFollowUp: generatedruntime.FollowUpSemantics{Strategy: "read-after-write"},
		DeleteFollowUp: generatedruntime.FollowUpSemantics{Strategy: "confirm-delete"},
	}
}

type wlpAgentStateFreeClient struct {
	delegate WlpAgentServiceClient
}

func wrapWlpAgentStateFreeClient(delegate WlpAgentServiceClient) WlpAgentServiceClient {
	return wlpAgentStateFreeClient{delegate: delegate}
}

func (c wlpAgentStateFreeClient) CreateOrUpdate(ctx context.Context, resource *cloudguardv1beta1.WlpAgent, req ctrl.Request) (servicemanager.OSOKResponse, error) {
	response, err := c.delegate.CreateOrUpdate(ctx, resource, req)
	if err != nil {
		return response, err
	}
	if response.IsSuccessful && response.ShouldRequeue && resource != nil && resource.Status.OsokStatus.Ocid != "" {
		markWlpAgentActive(resource)
		response.ShouldRequeue = false
		response.RequeueDuration = 0
	}
	return response, nil
}

func (c wlpAgentStateFreeClient) Delete(ctx context.Context, resource *cloudguardv1beta1.WlpAgent) (bool, error) {
	return c.delegate.Delete(ctx, resource)
}

func markWlpAgentActive(resource *cloudguardv1beta1.WlpAgent) {
	now := metav1.Now()
	status := &resource.Status.OsokStatus
	status.Async.Current = nil
	status.Message = "OCI WLP agent is active"
	status.Reason = string(shared.Active)
	status.UpdatedAt = &now
	*status = util.UpdateOSOKStatusCondition(*status, shared.Active, v1.ConditionTrue, "", status.Message, loggerutil.OSOKLogger{})
}
