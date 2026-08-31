/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// CoverageResource describes replay coverage for one checked-in controller.
type CoverageResource struct {
	Service        string             `json:"service"`
	Resource       string             `json:"resource"`
	ControllerPath string             `json:"controllerPath"`
	Cassettes      []CoverageCassette `json:"cassettes,omitempty"`
}

// CoverageCassette describes one cassette associated with a controller.
type CoverageCassette struct {
	Path     string   `json:"path"`
	Metadata Metadata `json:"metadata"`
}

// CoverageReport is a reporting-only inventory of controller replay coverage.
type CoverageReport struct {
	TotalControllers      int                `json:"totalControllers"`
	CoveredResources      int                `json:"coveredResources"`
	RecordedResources     int                `json:"recordedResources"`
	SyntheticResources    int                `json:"syntheticResources"`
	Resources             []CoverageResource `json:"resources"`
	Missing               []CoverageResource `json:"missing"`
	LegacyCassettes       []string           `json:"legacyCassettes,omitempty"`
	UnreferencedCassettes []string           `json:"unreferencedCassettes,omitempty"`
	OrphanCassettes       []CoverageCassette `json:"orphanCassettes,omitempty"`
}

// AuditCoverage inventories checked-in controllers and sanitized replay
// cassettes. Missing coverage is reported but is not an error.
func AuditCoverage(root string) (CoverageReport, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return CoverageReport{}, fmt.Errorf("resolve replay coverage root: %w", err)
	}
	resources, byKey, err := discoverControllerResources(root)
	if err != nil {
		return CoverageReport{}, err
	}
	legacy, unreferenced, orphans, err := discoverCoverageCassettes(root, resources, byKey)
	if err != nil {
		return CoverageReport{}, err
	}

	report := CoverageReport{
		TotalControllers:      len(resources),
		Resources:             resources,
		LegacyCassettes:       legacy,
		UnreferencedCassettes: unreferenced,
		OrphanCassettes:       orphans,
	}
	for index := range report.Resources {
		resource := &report.Resources[index]
		sort.Slice(resource.Cassettes, func(i, j int) bool { return resource.Cassettes[i].Path < resource.Cassettes[j].Path })
		if len(resource.Cassettes) == 0 {
			report.Missing = append(report.Missing, *resource)
			continue
		}
		report.CoveredResources++
		hasRecorded := false
		hasSynthetic := false
		for _, cassette := range resource.Cassettes {
			hasRecorded = hasRecorded || cassette.Metadata.Provenance == ProvenanceRecorded
			hasSynthetic = hasSynthetic || cassette.Metadata.Provenance == ProvenanceSynthetic
		}
		if hasRecorded {
			report.RecordedResources++
		}
		if hasSynthetic {
			report.SyntheticResources++
		}
	}
	return report, nil
}

func discoverControllerResources(root string) ([]CoverageResource, map[string]int, error) {
	controllersRoot := filepath.Join(root, "controllers")
	entries, err := os.ReadDir(controllersRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("read controllers directory: %w", err)
	}
	var resources []CoverageResource
	for _, serviceEntry := range entries {
		if !serviceEntry.IsDir() {
			continue
		}
		service := serviceEntry.Name()
		files, err := os.ReadDir(filepath.Join(controllersRoot, service))
		if err != nil {
			return nil, nil, fmt.Errorf("read controller service %s: %w", service, err)
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), "_controller.go") {
				continue
			}
			resource := strings.TrimSuffix(file.Name(), "_controller.go")
			resources = append(resources, CoverageResource{
				Service:        service,
				Resource:       resource,
				ControllerPath: filepath.ToSlash(filepath.Join("controllers", service, file.Name())),
			})
		}
	}
	sort.Slice(resources, func(i, j int) bool {
		return coverageKey(resources[i].Service, resources[i].Resource) < coverageKey(resources[j].Service, resources[j].Resource)
	})
	byKey := make(map[string]int, len(resources))
	for index, resource := range resources {
		byKey[coverageKey(resource.Service, resource.Resource)] = index
	}
	return resources, byKey, nil
}

func discoverCoverageCassettes(root string, controllerResources []CoverageResource, resourcesByKey map[string]int) ([]string, []string, []CoverageCassette, error) {
	serviceManagerRoot := filepath.Join(root, "pkg", "servicemanager")
	var legacy []string
	var unreferenced []string
	var orphans []CoverageCassette
	err := filepath.WalkDir(serviceManagerRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".yaml" || filepath.Base(filepath.Dir(path)) != "recordings" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		file, err := decodeCassetteFile(path)
		if err != nil {
			return fmt.Errorf("read replay coverage cassette %s: %w", relative, err)
		}
		if file.Metadata == nil {
			legacy = append(legacy, relative)
			return nil
		}
		if err := validateMetadata(*file.Metadata); err != nil {
			return fmt.Errorf("validate replay coverage cassette %s: %w", relative, err)
		}
		referenced, err := cassetteReferencedByReplayTest(path, file.Metadata.Provenance)
		if err != nil {
			return fmt.Errorf("inspect replay coverage cassette %s references: %w", relative, err)
		}
		if !referenced {
			unreferenced = append(unreferenced, relative)
			return nil
		}
		cassette := CoverageCassette{Path: relative, Metadata: *cloneMetadata(file.Metadata)}
		key := coverageKey(file.Metadata.Service, file.Metadata.Resource)
		index, ok := resourcesByKey[key]
		if !ok {
			orphans = append(orphans, cassette)
			return nil
		}
		controllerResources[index].Cassettes = append(controllerResources[index].Cassettes, cassette)
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("walk replay cassettes: %w", err)
	}
	sort.Strings(legacy)
	sort.Strings(unreferenced)
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Path < orphans[j].Path })
	return legacy, unreferenced, orphans, nil
}

func cassetteReferencedByReplayTest(path string, provenance Provenance) (bool, error) {
	packageDir := filepath.Dir(filepath.Dir(filepath.Dir(path)))
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		return false, err
	}
	name := []byte(filepath.Base(path))
	for _, entry := range entries {
		if entry.IsDir() || !isReplayIntegrationTest(entry.Name(), provenance) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(packageDir, entry.Name()))
		if err != nil {
			return false, err
		}
		if bytes.Contains(content, name) {
			return true, nil
		}
	}
	return false, nil
}

func isReplayIntegrationTest(name string, provenance Provenance) bool {
	switch provenance {
	case ProvenanceRecorded:
		return strings.HasSuffix(name, "_recorded_integration_test.go")
	case ProvenanceSynthetic:
		return strings.HasSuffix(name, "_synthetic_integration_test.go")
	default:
		return false
	}
}

func decodeCassetteFile(path string) (cassetteFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return cassetteFile{}, err
	}
	if err := validateSafeRecording(content); err != nil {
		return cassetteFile{}, err
	}
	var file cassetteFile
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return cassetteFile{}, err
	}
	if file.Version != Version {
		return cassetteFile{}, fmt.Errorf("cassette version %d is unsupported; expected %d", file.Version, Version)
	}
	if len(file.Interactions) == 0 {
		return cassetteFile{}, fmt.Errorf("cassette contains no interactions")
	}
	return file, nil
}

func coverageKey(service, resource string) string {
	return strings.ToLower(strings.TrimSpace(service)) + "/" + strings.ToLower(strings.TrimSpace(resource))
}
