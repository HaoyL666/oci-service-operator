/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package configurationsourceprovider

import (
	"testing"

	resourcemanagersdk "github.com/oracle/oci-go-sdk/v65/resourcemanager"
)

func TestConfigurationSourceProviderRuntimeCompatibilityUsesEmailMutation(t *testing.T) {
	t.Parallel()

	hooks := newConfigurationSourceProviderDefaultRuntimeHooks(resourcemanagersdk.ResourceManagerClient{})
	applyConfigurationSourceProviderRuntimeCompatibility(nil, &hooks)

	if !containsConfigurationSourceProviderField(hooks.Semantics.Mutation.Mutable, "email") {
		t.Fatalf("mutable fields = %#v, want email", hooks.Semantics.Mutation.Mutable)
	}
	if containsConfigurationSourceProviderField(hooks.Semantics.Mutation.Mutable, "username") {
		t.Fatalf("mutable fields = %#v, do not want removed username", hooks.Semantics.Mutation.Mutable)
	}
}

func containsConfigurationSourceProviderField(fields []string, want string) bool {
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}
