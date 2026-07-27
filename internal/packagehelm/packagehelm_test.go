package packagehelm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseControllerImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      string
		repository string
		tag        string
		digest     string
		wantError  bool
	}{
		{
			name:       "tag",
			value:      "ghcr.io/oracle/oci-service-operator-psql:v2.3.0-alpha",
			repository: "ghcr.io/oracle/oci-service-operator-psql",
			tag:        "v2.3.0-alpha",
		},
		{
			name:       "registry port",
			value:      "localhost:5000/osok/psql:v2.3.0",
			repository: "localhost:5000/osok/psql",
			tag:        "v2.3.0",
		},
		{
			name:       "digest",
			value:      "ghcr.io/oracle/oci-service-operator-psql@sha256:" + strings.Repeat("a", 64),
			repository: "ghcr.io/oracle/oci-service-operator-psql",
			digest:     "sha256:" + strings.Repeat("a", 64),
		},
		{name: "missing tag", value: "ghcr.io/oracle/psql", wantError: true},
		{name: "bad digest", value: "ghcr.io/oracle/psql@sha256:not-a-digest", wantError: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseControllerImage(test.value)
			if test.wantError {
				if err == nil {
					t.Fatalf("parseControllerImage(%q) unexpectedly succeeded", test.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseControllerImage(%q): %v", test.value, err)
			}
			if got.Repository != test.repository || got.Tag != test.tag || got.Digest != test.digest {
				t.Fatalf("parseControllerImage(%q) = %#v", test.value, got)
			}
		})
	}
}

func TestValidateGenerateOptionsSupportsServiceAndSplitGroups(t *testing.T) {
	t.Parallel()

	for _, group := range []string{"queue", "core-network"} {
		group := group
		t.Run(group, func(t *testing.T) {
			t.Parallel()
			tempDir := t.TempDir()
			opts := GenerateOptions{
				Group:           group,
				Version:         "v2.3.0-alpha",
				ControllerImage: "ghcr.io/oracle/oci-service-operator-" + group + ":v2.3.0-alpha",
				ManifestPath:    filepath.Join(tempDir, "package.yaml"),
				SkeletonDir:     filepath.Join(tempDir, "skeleton"),
				OutputDir:       filepath.Join(tempDir, chartName(group)),
				CRDOutputPath:   filepath.Join(tempDir, group+"-crds-2.3.0-alpha.yaml"),
			}
			if err := validateGenerateOptions(opts); err != nil {
				t.Fatalf("validateGenerateOptions(%q): %v", group, err)
			}
		})
	}
}

func TestValidateGenerateOptionsRejectsMismatchedArtifactNames(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	valid := GenerateOptions{
		Group:           "queue",
		Version:         "v2.3.0",
		ControllerImage: "ghcr.io/oracle/oci-service-operator-queue:v2.3.0",
		ManifestPath:    filepath.Join(tempDir, "package.yaml"),
		SkeletonDir:     filepath.Join(tempDir, "skeleton"),
		OutputDir:       filepath.Join(tempDir, chartName("queue")),
		CRDOutputPath:   filepath.Join(tempDir, "queue-crds-2.3.0.yaml"),
	}

	tests := []struct {
		name   string
		mutate func(*GenerateOptions)
	}{
		{name: "invalid group", mutate: func(opts *GenerateOptions) { opts.Group = "Queue" }},
		{name: "wrong chart name", mutate: func(opts *GenerateOptions) { opts.OutputDir = filepath.Join(tempDir, "psql-chart") }},
		{name: "wrong CRD prefix", mutate: func(opts *GenerateOptions) { opts.CRDOutputPath = filepath.Join(tempDir, "psql-crds-2.3.0.yaml") }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			opts := valid
			test.mutate(&opts)
			if err := validateGenerateOptions(opts); err == nil {
				t.Fatalf("validateGenerateOptions() unexpectedly accepted %s", test.name)
			}
		})
	}
}

func TestRenderTemplateDirectoryRendersAndRenamesTemplates(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "Chart.yaml.tmpl")
	readmePath := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(templatePath, []byte("name: __OSOK_CHART_NAME__\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readmePath, []byte("# __OSOK_GROUP__\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := renderTemplateDirectory(tempDir, map[string]string{
		"__OSOK_CHART_NAME__": "oci-service-operator-queue-chart",
		"__OSOK_GROUP__":      "queue",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(templatePath); !os.IsNotExist(err) {
		t.Fatalf("template source still exists: %v", err)
	}
	chart, err := os.ReadFile(filepath.Join(tempDir, "Chart.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(chart) != "name: oci-service-operator-queue-chart\n" {
		t.Fatalf("rendered Chart.yaml = %q", chart)
	}
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(readme) != "# queue\n" {
		t.Fatalf("rendered README.md = %q", readme)
	}
}

func TestVerifyManifestParityNormalizesIntentionalDifferences(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	packagePath := filepath.Join(tempDir, "package.yaml")
	helmPath := filepath.Join(tempDir, "helm.yaml")
	packageManifest := `apiVersion: v1
kind: Namespace
metadata:
  name: osok-system
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: examples.example.com
spec:
  group: example.com
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: manager-config
  namespace: test
data:
  controller_manager_config.yaml: |
    kind: ControllerManagerConfiguration
    health:
      healthProbeBindAddress: :8081
`
	helmManifest := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    argocd.argoproj.io/sync-options: Prune=false
  name: examples.example.com
spec:
  group: example.com
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: manager-config
  namespace: test
data:
  controller_manager_config.yaml: |
    health:
      healthProbeBindAddress: :8081
    kind: ControllerManagerConfiguration
`
	if err := os.WriteFile(packagePath, []byte(packageManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helmPath, []byte(helmManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyManifestParity(packagePath, helmPath); err != nil {
		t.Fatalf("VerifyManifestParity() returned error: %v", err)
	}
}

func TestVerifyManifestParityNormalizesDedicatedServiceAccount(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	packagePath := filepath.Join(tempDir, "package.yaml")
	helmPath := filepath.Join(tempDir, "helm.yaml")
	packageManifest := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: oci-service-operator-example-controller-manager
  namespace: test
spec:
  template:
    spec:
      containers: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: manager-rolebinding
  namespace: test
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: manager-role
subjects:
- kind: ServiceAccount
  name: default
  namespace: test
`
	helmManifest := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: oci-service-operator-example-controller-manager
  namespace: test
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: oci-service-operator-example-controller-manager
  namespace: test
spec:
  template:
    spec:
      serviceAccountName: oci-service-operator-example-controller-manager
      containers: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: manager-rolebinding
  namespace: test
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: manager-role
subjects:
- kind: ServiceAccount
  name: oci-service-operator-example-controller-manager
  namespace: test
`
	if err := os.WriteFile(packagePath, []byte(packageManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helmPath, []byte(helmManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyManifestParity(packagePath, helmPath); err != nil {
		t.Fatalf("VerifyManifestParity() returned error: %v", err)
	}
}

func TestVerifyManifestParityDoesNotIgnoreUnrelatedServiceAccounts(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	packagePath := filepath.Join(tempDir, "package.yaml")
	helmPath := filepath.Join(tempDir, "helm.yaml")
	packageManifest := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: workload
  namespace: test
`
	if err := os.WriteFile(packagePath, []byte(packageManifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helmPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyManifestParity(packagePath, helmPath); err == nil {
		t.Fatal("VerifyManifestParity() unexpectedly ignored an unrelated ServiceAccount")
	}
}

func TestVerifySecurityPostureRequiresPrunePolicyOnEveryCRD(t *testing.T) {
	t.Parallel()

	manifestPath := filepath.Join(t.TempDir(), "helm.yaml")
	manifest := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    argocd.argoproj.io/sync-options: Prune=false
  name: protected.example.com
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: unprotected.example.com
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
spec:
  template:
    spec:
      serviceAccountName: controller-manager
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: manager
          image: ghcr.io/oracle/oci-service-operator-example:v1.0.0
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	err := VerifySecurityPosture(manifestPath)
	if err == nil {
		t.Fatal("VerifySecurityPosture() unexpectedly accepted a CRD without the prune policy")
	}
	if !strings.Contains(err.Error(), `CRD "unprotected.example.com"`) {
		t.Fatalf("VerifySecurityPosture() error = %q", err)
	}
}

func TestWriteChecksum(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "artifact.tgz")
	if err := os.WriteFile(path, []byte("osok"), 0o600); err != nil {
		t.Fatal(err)
	}
	checksum, err := WriteChecksum(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(checksum) != 64 {
		t.Fatalf("checksum length = %d, want 64", len(checksum))
	}
	content, err := os.ReadFile(path + ".sha256")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "artifact.tgz") {
		t.Fatalf("checksum file %q does not name artifact", content)
	}
}

func TestWriteChartArchiveIsDeterministic(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	chartDir := filepath.Join(tempDir, "example-chart")
	if err := os.MkdirAll(filepath.Join(chartDir, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: example-chart\nversion: 1.0.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "templates", "config.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	firstPath := filepath.Join(tempDir, "first.tgz")
	secondPath := filepath.Join(tempDir, "second.tgz")
	if err := WriteChartArchive(chartDir, firstPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(chartDir, "Chart.yaml"), time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := WriteChartArchive(chartDir, secondPath); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("identical chart content produced different archives")
	}
}
