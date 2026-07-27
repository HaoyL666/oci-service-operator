package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/oracle/oci-service-operator/internal/packagehelm"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "packagehelm: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return fmt.Errorf("command is required")
	}
	switch args[0] {
	case "generate":
		return runGenerate(args[1:])
	case "verify":
		return runVerify(args[1:])
	case "checksum":
		return runChecksum(args[1:])
	case "archive":
		return runArchive(args[1:])
	case "security":
		return runSecurity(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runSecurity(args []string) error {
	var helmManifestPath string
	flags := flag.NewFlagSet("packagehelm security", flag.ContinueOnError)
	flags.StringVar(&helmManifestPath, "helm-manifest", "", "Rendered Helm manifest including CRDs")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if helmManifestPath == "" {
		return fmt.Errorf("helm-manifest is required")
	}
	if err := packagehelm.VerifySecurityPosture(helmManifestPath); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Helm security posture checks passed")
	return nil
}

func runArchive(args []string) error {
	var chartDir string
	var outputPath string
	flags := flag.NewFlagSet("packagehelm archive", flag.ContinueOnError)
	flags.StringVar(&chartDir, "chart-dir", "", "Generated chart directory")
	flags.StringVar(&outputPath, "output", "", "Destination .tgz path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if chartDir == "" || outputPath == "" {
		return fmt.Errorf("chart-dir and output are required")
	}
	if err := packagehelm.WriteChartArchive(chartDir, outputPath); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "packaged deterministic Helm chart at %s\n", outputPath)
	return nil
}

func runGenerate(args []string) error {
	opts := packagehelm.GenerateOptions{}
	flags := flag.NewFlagSet("packagehelm generate", flag.ContinueOnError)
	flags.StringVar(&opts.Group, "group", "", "Package group")
	flags.StringVar(&opts.Version, "version", "", "Release version including its leading v")
	flags.StringVar(&opts.ControllerImage, "controller-image", "", "Exact controller image reference")
	flags.StringVar(&opts.ManifestPath, "manifest", "", "Rendered package manifest")
	flags.StringVar(&opts.SkeletonDir, "skeleton", "", "Reviewed chart skeleton directory")
	flags.StringVar(&opts.OutputDir, "output-dir", "", "Generated chart directory")
	flags.StringVar(&opts.CRDOutputPath, "crd-output", "", "Versioned companion CRD manifest")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := packagehelm.Generate(opts); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "generated Helm chart at %s\n", opts.OutputDir)
	return nil
}

func runVerify(args []string) error {
	var packageManifestPath string
	var helmManifestPath string
	flags := flag.NewFlagSet("packagehelm verify", flag.ContinueOnError)
	flags.StringVar(&packageManifestPath, "package-manifest", "", "Rendered package manifest")
	flags.StringVar(&helmManifestPath, "helm-manifest", "", "Rendered Helm manifest including CRDs")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if packageManifestPath == "" || helmManifestPath == "" {
		return fmt.Errorf("package-manifest and helm-manifest are required")
	}
	if err := packagehelm.VerifyManifestParity(packageManifestPath, helmManifestPath); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "package and Helm manifests are semantically equivalent")
	return nil
}

func runChecksum(args []string) error {
	var path string
	flags := flag.NewFlagSet("packagehelm checksum", flag.ContinueOnError)
	flags.StringVar(&path, "file", "", "Artifact to checksum")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("file is required")
	}
	checksum, err := packagehelm.WriteChecksum(path)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s  %s\n", checksum, path)
	return nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: packagehelm <generate|verify|security|archive|checksum> [flags]")
}
