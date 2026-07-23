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
