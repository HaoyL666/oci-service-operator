/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/oracle/oci-service-operator/internal/e2e/ocireplay"
)

func main() {
	root := flag.String("root", ".", "repository root")
	jsonOutput := flag.Bool("json", false, "emit the coverage report as JSON")
	verbose := flag.Bool("verbose", false, "list missing, legacy, and orphan entries")
	flag.Parse()

	report, err := ocireplay.AuditCoverage(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "replay coverage audit failed: %v\n", err)
		os.Exit(1)
	}
	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "encode replay coverage report: %v\n", err)
			os.Exit(1)
		}
		return
	}

	percentage := 0.0
	if report.TotalControllers > 0 {
		percentage = float64(report.CoveredResources) / float64(report.TotalControllers) * 100
	}
	fmt.Printf("OCI replay coverage: %d/%d controller resources (%.1f%%)\n", report.CoveredResources, report.TotalControllers, percentage)
	fmt.Printf("  recorded resources:  %d\n", report.RecordedResources)
	fmt.Printf("  synthetic resources: %d\n", report.SyntheticResources)
	fmt.Printf("  missing resources:   %d\n", len(report.Missing))
	fmt.Printf("  legacy cassettes:    %d\n", len(report.LegacyCassettes))
	fmt.Printf("  unreferenced cassettes: %d\n", len(report.UnreferencedCassettes))
	fmt.Printf("  orphan cassettes:    %d\n", len(report.OrphanCassettes))
	fmt.Printf("OCI replay classifications: %d/%d controller resources\n", report.ClassifiedResources, report.TotalControllers)
	fmt.Printf("  recorded strategy:   %d\n", report.RecordedClassified)
	fmt.Printf("  synthetic strategy:  %d\n", report.SyntheticClassified)
	fmt.Printf("  deferred resources:  %d\n", report.DeferredClassified)
	fmt.Printf("  unclassified:        %d\n", len(report.Unclassified))
	if *verbose {
		for _, resource := range report.Missing {
			fmt.Printf(
				"missing %s/%s classification=%s (%s)\n",
				resource.Service,
				resource.Resource,
				resource.Classification,
				resource.ControllerPath,
			)
		}
		for _, resource := range report.Deferred {
			fmt.Printf(
				"deferred %s/%s blocker=%q next=%q\n",
				resource.Service,
				resource.Resource,
				resource.Blocker,
				resource.NextAction,
			)
		}
		for _, resource := range report.Unclassified {
			fmt.Printf("unclassified %s/%s (%s)\n", resource.Service, resource.Resource, resource.ControllerPath)
		}
		for _, path := range report.LegacyCassettes {
			fmt.Printf("legacy %s\n", path)
		}
		for _, path := range report.UnreferencedCassettes {
			fmt.Printf("unreferenced %s\n", path)
		}
		for _, cassette := range report.OrphanCassettes {
			fmt.Printf("orphan %s (%s/%s)\n", cassette.Path, cassette.Metadata.Service, cassette.Metadata.Resource)
		}
	}
	fmt.Println("Coverage and classification completeness are reporting-only; missing or unclassified resources do not fail this milestone.")
}
