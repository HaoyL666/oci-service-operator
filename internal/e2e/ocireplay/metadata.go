/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var metadataServicePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var metadataResourcePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

func validateMetadata(metadata Metadata) error {
	service := strings.TrimSpace(metadata.Service)
	resource := strings.TrimSpace(metadata.Resource)
	sdkVersion := strings.TrimSpace(metadata.SDKVersion)
	if service != metadata.Service {
		return fmt.Errorf("cassette metadata service must not contain surrounding whitespace")
	}
	if resource != metadata.Resource {
		return fmt.Errorf("cassette metadata resource must not contain surrounding whitespace")
	}
	if sdkVersion != metadata.SDKVersion {
		return fmt.Errorf("cassette metadata sdkVersion must not contain surrounding whitespace")
	}
	if !metadataServicePattern.MatchString(service) {
		return fmt.Errorf("cassette metadata service %q must use lowercase letters, digits, or hyphens", metadata.Service)
	}
	if resource == "" {
		return fmt.Errorf("cassette metadata resource is required")
	}
	if !metadataResourcePattern.MatchString(resource) {
		return fmt.Errorf("cassette metadata resource %q must be a Kubernetes kind name", metadata.Resource)
	}
	if sdkVersion == "" {
		return fmt.Errorf("cassette metadata sdkVersion is required")
	}
	switch metadata.Provenance {
	case ProvenanceRecorded, ProvenanceSynthetic:
	default:
		return fmt.Errorf("cassette metadata provenance %q is unsupported; expected %q or %q", metadata.Provenance, ProvenanceRecorded, ProvenanceSynthetic)
	}
	if len(metadata.Operations) == 0 {
		return fmt.Errorf("cassette metadata operations must not be empty")
	}
	seen := make(map[Operation]bool, len(metadata.Operations))
	for index, operation := range metadata.Operations {
		switch operation {
		case OperationCreate, OperationRead, OperationUpdate, OperationDelete, OperationAction:
		default:
			return fmt.Errorf("cassette metadata operations[%d] %q is unsupported", index, operation)
		}
		if seen[operation] {
			return fmt.Errorf("cassette metadata operation %q is duplicated", operation)
		}
		seen[operation] = true
	}
	return nil
}

func cloneMetadata(metadata *Metadata) *Metadata {
	if metadata == nil {
		return nil
	}
	cloned := *metadata
	cloned.Operations = append([]Operation(nil), metadata.Operations...)
	return &cloned
}

func metadataEqual(left, right Metadata) bool {
	if left.Service != right.Service || left.Resource != right.Resource ||
		left.SDKVersion != right.SDKVersion || left.Provenance != right.Provenance ||
		len(left.Operations) != len(right.Operations) {
		return false
	}
	leftOperations := append([]Operation(nil), left.Operations...)
	rightOperations := append([]Operation(nil), right.Operations...)
	sort.Slice(leftOperations, func(i, j int) bool { return leftOperations[i] < leftOperations[j] })
	sort.Slice(rightOperations, func(i, j int) bool { return rightOperations[i] < rightOperations[j] })
	for index := range leftOperations {
		if leftOperations[index] != rightOperations[index] {
			return false
		}
	}
	return true
}
