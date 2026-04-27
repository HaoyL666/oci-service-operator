/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package alarmsuppression

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	monitoringsdk "github.com/oracle/oci-go-sdk/v65/monitoring"
	monitoringv1beta1 "github.com/oracle/oci-service-operator/api/monitoring/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/credhelper"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func init() {
	registerAlarmSuppressionRuntimeHooksMutator(func(manager *AlarmSuppressionServiceManager, hooks *AlarmSuppressionRuntimeHooks) {
		applyAlarmSuppressionRuntimeHooks(manager, hooks)
	})
}

func applyAlarmSuppressionRuntimeHooks(
	manager *AlarmSuppressionServiceManager,
	hooks *AlarmSuppressionRuntimeHooks,
) {
	if manager == nil || hooks == nil {
		return
	}

	hooks.BuildCreateBody = func(
		ctx context.Context,
		resource *monitoringv1beta1.AlarmSuppression,
		namespace string,
	) (any, error) {
		return buildAlarmSuppressionCreateDetails(ctx, manager.CredentialClient, resource, namespace)
	}
}

func buildAlarmSuppressionCreateDetails(
	ctx context.Context,
	credentialClient credhelper.CredentialClient,
	resource *monitoringv1beta1.AlarmSuppression,
	namespace string,
) (monitoringsdk.CreateAlarmSuppressionDetails, error) {
	resolvedSpec, err := generatedruntime.ResolveSpecValue(resource, ctx, credentialClient, namespace)
	if err != nil {
		return monitoringsdk.CreateAlarmSuppressionDetails{}, err
	}

	payload, err := json.Marshal(resolvedSpec)
	if err != nil {
		return monitoringsdk.CreateAlarmSuppressionDetails{}, fmt.Errorf("marshal resolved alarm suppression spec: %w", err)
	}

	var details monitoringsdk.CreateAlarmSuppressionDetails
	if err := json.Unmarshal(payload, &details); err != nil {
		return monitoringsdk.CreateAlarmSuppressionDetails{}, fmt.Errorf("decode alarm suppression create request body: %w", err)
	}

	if level := strings.TrimSpace(resource.Spec.Level); level != "" {
		details.Level = monitoringsdk.AlarmSuppressionLevelEnum(level)
	} else {
		details.Level = ""
	}
	details.Dimensions = resource.Spec.Dimensions
	if len(resource.Spec.SuppressionConditions) == 0 {
		details.SuppressionConditions = nil
	} else {
		details.SuppressionConditions = append([]monitoringsdk.SuppressionCondition(nil), details.SuppressionConditions...)
	}

	return details, nil
}
