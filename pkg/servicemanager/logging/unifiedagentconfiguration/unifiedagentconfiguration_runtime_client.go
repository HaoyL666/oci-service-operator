/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package unifiedagentconfiguration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	loggingsdk "github.com/oracle/oci-go-sdk/v65/logging"
	loggingv1beta1 "github.com/oracle/oci-service-operator/api/logging/v1beta1"
	"github.com/oracle/oci-service-operator/pkg/credhelper"
	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

func init() {
	registerUnifiedAgentConfigurationRuntimeHooksMutator(func(manager *UnifiedAgentConfigurationServiceManager, hooks *UnifiedAgentConfigurationRuntimeHooks) {
		applyUnifiedAgentConfigurationRuntimeHooks(manager, hooks)
	})
}

func applyUnifiedAgentConfigurationRuntimeHooks(
	manager *UnifiedAgentConfigurationServiceManager,
	hooks *UnifiedAgentConfigurationRuntimeHooks,
) {
	if manager == nil || hooks == nil {
		return
	}

	hooks.BuildCreateBody = func(
		ctx context.Context,
		resource *loggingv1beta1.UnifiedAgentConfiguration,
		namespace string,
	) (any, error) {
		return buildUnifiedAgentConfigurationCreateDetails(ctx, manager.CredentialClient, resource, namespace)
	}
}

func buildUnifiedAgentConfigurationCreateDetails(
	ctx context.Context,
	credentialClient credhelper.CredentialClient,
	resource *loggingv1beta1.UnifiedAgentConfiguration,
	namespace string,
) (loggingsdk.CreateUnifiedAgentConfigurationDetails, error) {
	resolvedSpec, err := generatedruntime.ResolveSpecValue(resource, ctx, credentialClient, namespace)
	if err != nil {
		return loggingsdk.CreateUnifiedAgentConfigurationDetails{}, err
	}

	payload, err := json.Marshal(resolvedSpec)
	if err != nil {
		return loggingsdk.CreateUnifiedAgentConfigurationDetails{}, fmt.Errorf("marshal resolved unified agent configuration spec: %w", err)
	}

	var details loggingsdk.CreateUnifiedAgentConfigurationDetails
	if err := json.Unmarshal(payload, &details); err != nil {
		return loggingsdk.CreateUnifiedAgentConfigurationDetails{}, fmt.Errorf("decode unified agent configuration create request body: %w", err)
	}

	// Keep the upgraded mandatory fields explicit in the handwritten seam so the
	// validator can prove the generated-runtime create path still projects them.
	details.DisplayName = common.String(resource.Spec.DisplayName)
	details.Description = common.String(resource.Spec.Description)

	return details, nil
}
