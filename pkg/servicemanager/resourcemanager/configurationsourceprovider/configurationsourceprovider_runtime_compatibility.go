/* Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved. */
package configurationsourceprovider

func init() {
	registerConfigurationSourceProviderRuntimeHooksMutator(applyConfigurationSourceProviderRuntimeCompatibility)
}

func applyConfigurationSourceProviderRuntimeCompatibility(
	_ *ConfigurationSourceProviderServiceManager,
	hooks *ConfigurationSourceProviderRuntimeHooks,
) {
	if hooks == nil || hooks.Semantics == nil {
		return
	}
	hooks.Semantics.Mutation.Mutable = replaceConfigurationSourceProviderMutableField(
		hooks.Semantics.Mutation.Mutable,
		"username",
		"email",
	)
}

func replaceConfigurationSourceProviderMutableField(fields []string, removed, added string) []string {
	updated := make([]string, 0, len(fields))
	containsAdded := false
	for _, field := range fields {
		if field == removed {
			continue
		}
		if field == added {
			containsAdded = true
		}
		updated = append(updated, field)
	}
	if !containsAdded {
		updated = append(updated, added)
	}
	return updated
}
