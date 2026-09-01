/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const coverageClassificationSchemaVersion = 1
const coverageClassificationPath = "internal/e2e/ocireplay/classifications.yaml"

// ReplayClassification describes the intended replay strategy for a
// controller resource. Unclassified is derived and cannot be declared.
type ReplayClassification string

const (
	ReplayClassificationRecorded     ReplayClassification = "recorded"
	ReplayClassificationSynthetic    ReplayClassification = "synthetic"
	ReplayClassificationDeferred     ReplayClassification = "deferred"
	ReplayClassificationUnclassified ReplayClassification = "unclassified"
)

type coverageClassificationFile struct {
	SchemaVersion int                                 `yaml:"schemaVersion"`
	Resources     []coverageClassificationDeclaration `yaml:"resources"`
}

type coverageClassificationDeclaration struct {
	Service         string               `yaml:"service"`
	Resource        string               `yaml:"resource"`
	Classification  ReplayClassification `yaml:"classification"`
	Reason          string               `yaml:"reason,omitempty"`
	SyntheticReason string               `yaml:"syntheticReason,omitempty"`
	Blocker         string               `yaml:"blocker,omitempty"`
	NextAction      string               `yaml:"nextAction,omitempty"`
}

func loadCoverageClassifications(
	root string,
) (map[string]coverageClassificationDeclaration, error) {
	path := filepath.Join(root, filepath.FromSlash(coverageClassificationPath))
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read required replay classifications %s: %w", coverageClassificationPath, err)
	}

	var file coverageClassificationFile
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode replay classifications: %w", err)
	}
	if file.SchemaVersion != coverageClassificationSchemaVersion {
		return nil, fmt.Errorf(
			"replay classification schemaVersion %d is unsupported; expected %d",
			file.SchemaVersion,
			coverageClassificationSchemaVersion,
		)
	}

	declarations := make(map[string]coverageClassificationDeclaration, len(file.Resources))
	keys := make([]string, 0, len(file.Resources))
	for index, declaration := range file.Resources {
		if err := validateClassificationDeclaration(declaration); err != nil {
			return nil, fmt.Errorf("replay classification resource %d: %w", index+1, err)
		}
		key := coverageKey(declaration.Service, declaration.Resource)
		if _, exists := declarations[key]; exists {
			return nil, fmt.Errorf("replay classification %s is declared more than once", key)
		}
		declarations[key] = declaration
		keys = append(keys, key)
	}
	if !sort.StringsAreSorted(keys) {
		return nil, fmt.Errorf("replay classification resources must be sorted by service/resource")
	}
	return declarations, nil
}

func validateClassificationDeclaration(declaration coverageClassificationDeclaration) error {
	service := strings.TrimSpace(declaration.Service)
	resource := strings.TrimSpace(declaration.Resource)
	if service == "" || resource == "" {
		return fmt.Errorf("service and resource are required")
	}
	reason := strings.TrimSpace(declaration.Reason)
	syntheticReason := strings.TrimSpace(declaration.SyntheticReason)
	blocker := strings.TrimSpace(declaration.Blocker)
	nextAction := strings.TrimSpace(declaration.NextAction)

	switch declaration.Classification {
	case ReplayClassificationRecorded:
		if blocker != "" {
			return fmt.Errorf("recorded classification %s/%s must not declare blocker", service, resource)
		}
	case ReplayClassificationSynthetic:
		if reason == "" {
			return fmt.Errorf("synthetic classification %s/%s requires reason", service, resource)
		}
		if syntheticReason != "" {
			return fmt.Errorf("synthetic classification %s/%s must use reason, not syntheticReason", service, resource)
		}
		if blocker != "" {
			return fmt.Errorf("synthetic classification %s/%s must not declare blocker", service, resource)
		}
	case ReplayClassificationDeferred:
		if reason == "" || blocker == "" || nextAction == "" {
			return fmt.Errorf(
				"deferred classification %s/%s requires reason, blocker, and nextAction",
				service,
				resource,
			)
		}
		if syntheticReason != "" {
			return fmt.Errorf("deferred classification %s/%s must not declare syntheticReason", service, resource)
		}
	case ReplayClassificationUnclassified:
		return fmt.Errorf("unclassified is derived and cannot be declared for %s/%s", service, resource)
	default:
		return fmt.Errorf(
			"classification %q for %s/%s is unsupported; expected recorded, synthetic, or deferred",
			declaration.Classification,
			service,
			resource,
		)
	}
	return nil
}

func classifyCoverageResource(
	resource *CoverageResource,
	declarations map[string]coverageClassificationDeclaration,
	hasRecorded bool,
	hasSynthetic bool,
) error {
	key := coverageKey(resource.Service, resource.Resource)
	declaration, declared := declarations[key]

	derived := ReplayClassificationUnclassified
	switch {
	case hasRecorded:
		derived = ReplayClassificationRecorded
	case hasSynthetic:
		derived = ReplayClassificationSynthetic
	case declared:
		derived = declaration.Classification
	}

	if declared && (hasRecorded || hasSynthetic) && declaration.Classification != derived {
		return fmt.Errorf(
			"replay classification %s declares %q but cassette coverage derives %q",
			key,
			declaration.Classification,
			derived,
		)
	}
	if hasSynthetic {
		if !declared {
			return fmt.Errorf("synthetic cassette coverage for %s requires a classification justification", key)
		}
		if hasRecorded && strings.TrimSpace(declaration.SyntheticReason) == "" {
			return fmt.Errorf("recorded resource %s with synthetic scenarios requires syntheticReason", key)
		}
	}
	if declared && len(resource.Cassettes) == 0 &&
		(declaration.Classification == ReplayClassificationRecorded ||
			declaration.Classification == ReplayClassificationSynthetic) &&
		strings.TrimSpace(declaration.NextAction) == "" {
		return fmt.Errorf("planned %s classification %s requires nextAction", declaration.Classification, key)
	}
	if declaration.Classification == ReplayClassificationDeferred && len(resource.Cassettes) != 0 {
		return fmt.Errorf("deferred classification %s conflicts with checked-in cassette coverage", key)
	}

	resource.Classification = derived
	if declared {
		resource.ClassificationReason = strings.TrimSpace(declaration.Reason)
		resource.SyntheticReason = strings.TrimSpace(declaration.SyntheticReason)
		resource.Blocker = strings.TrimSpace(declaration.Blocker)
		resource.NextAction = strings.TrimSpace(declaration.NextAction)
	}
	return nil
}

func validateClassificationTargets(
	resources []CoverageResource,
	declarations map[string]coverageClassificationDeclaration,
) error {
	known := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		known[coverageKey(resource.Service, resource.Resource)] = struct{}{}
	}
	var unknown []string
	for key := range declarations {
		if _, ok := known[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("replay classifications reference unknown controllers: %s", strings.Join(unknown, ", "))
}
