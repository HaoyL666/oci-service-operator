package packagehelm

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

const (
	argocdSyncOptionsAnnotation = "argocd.argoproj.io/sync-options"
	argocdPruneFalse            = "Prune=false"
	helmNamespaceToken          = "__OSOK_HELM_RELEASE_NAMESPACE__"
	serviceAccountToken         = "__OSOK_SERVICE_ACCOUNT__"
)

var (
	chartVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	digestPattern       = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	groupPattern        = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
	imageTagPattern     = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
)

var deterministicArchiveTime = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)

// GenerateOptions identifies the source package manifest and the destination
// used to assemble one service-scoped Helm chart.
type GenerateOptions struct {
	Group           string
	Version         string
	ControllerImage string
	ManifestPath    string
	SkeletonDir     string
	OutputDir       string
	CRDOutputPath   string
}

// Generate assembles a deterministic Helm chart from a reviewed chart skeleton
// and the source package manifest. RBAC and CRDs are always sourced from the
// generated package output so those security-sensitive resources cannot drift.
func Generate(opts GenerateOptions) error {
	if err := validateGenerateOptions(opts); err != nil {
		return err
	}

	chartVersion := strings.TrimPrefix(opts.Version, "v")
	image, err := parseControllerImage(opts.ControllerImage)
	if err != nil {
		return err
	}
	if image.Digest == "" && image.Tag != opts.Version {
		return fmt.Errorf("controller image tag %q must equal release version %q", image.Tag, opts.Version)
	}

	objects, err := readObjects(opts.ManifestPath)
	if err != nil {
		return fmt.Errorf("read package manifest: %w", err)
	}

	if err := resetGeneratedDirectory(opts.OutputDir); err != nil {
		return err
	}
	if err := copyDirectory(opts.SkeletonDir, opts.OutputDir); err != nil {
		return fmt.Errorf("copy chart skeleton: %w", err)
	}

	namePrefix := "oci-service-operator-" + opts.Group + "-"
	replacements := map[string]string{
		"__OSOK_GROUP__":                     opts.Group,
		"__OSOK_CHART_NAME__":                chartName(opts.Group),
		"__OSOK_MANAGER_NAME__":              namePrefix + "controller-manager",
		"__OSOK_MANAGER_CONFIG_NAME__":       namePrefix + "manager-config",
		"__OSOK_CONFIG_SECRET_NAME__":        namePrefix + "osokconfig",
		"__OSOK_MANAGER_ROLE_NAME__":         namePrefix + "manager-role",
		"__OSOK_MANAGER_ROLE_BINDING_NAME__": namePrefix + "manager-rolebinding",
		"__OSOK_LEADER_ROLE_NAME__":          namePrefix + "leader-election-role",
		"__OSOK_LEADER_ROLE_BINDING_NAME__":  namePrefix + "leader-election-rolebinding",
		"__OSOK_CHART_VERSION__":             chartVersion,
		"__OSOK_APP_VERSION__":               opts.Version,
		"__OSOK_IMAGE_REPOSITORY__":          image.Repository,
		"__OSOK_IMAGE_TAG__":                 image.Tag,
		"__OSOK_IMAGE_DIGEST__":              image.Digest,
	}
	if err := renderTemplateDirectory(opts.OutputDir, replacements); err != nil {
		return err
	}

	if err := generateRBAC(objects, filepath.Join(opts.OutputDir, "templates", "generated-rbac.yaml")); err != nil {
		return err
	}
	if err := generateCRDs(objects, filepath.Join(opts.OutputDir, "crds"), opts.CRDOutputPath); err != nil {
		return err
	}

	return nil
}

// VerifyManifestParity compares the package output with Helm-rendered output.
// The package Namespace, chart-owned dedicated ServiceAccount, and the chart's
// Argo CD CRD prune annotation are intentional differences.
func VerifyManifestParity(packageManifestPath, helmManifestPath string) error {
	packageObjects, err := readObjects(packageManifestPath)
	if err != nil {
		return fmt.Errorf("read package manifest: %w", err)
	}
	helmObjects, err := readObjects(helmManifestPath)
	if err != nil {
		return fmt.Errorf("read Helm manifest: %w", err)
	}

	packageByKey, err := normalizeObjects(packageObjects, true)
	if err != nil {
		return fmt.Errorf("normalize package manifest: %w", err)
	}
	helmByKey, err := normalizeObjects(helmObjects, false)
	if err != nil {
		return fmt.Errorf("normalize Helm manifest: %w", err)
	}

	keys := make(map[string]struct{}, len(packageByKey)+len(helmByKey))
	for key := range packageByKey {
		keys[key] = struct{}{}
	}
	for key := range helmByKey {
		keys[key] = struct{}{}
	}
	sortedKeys := make([]string, 0, len(keys))
	for key := range keys {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	var differences []string
	for _, key := range sortedKeys {
		packageObject, packageOK := packageByKey[key]
		helmObject, helmOK := helmByKey[key]
		switch {
		case !packageOK:
			differences = append(differences, fmt.Sprintf("%s exists only in Helm output", key))
		case !helmOK:
			differences = append(differences, fmt.Sprintf("%s exists only in package output", key))
		case !reflect.DeepEqual(packageObject, helmObject):
			packageJSON, _ := json.MarshalIndent(packageObject, "", "  ")
			helmJSON, _ := json.MarshalIndent(helmObject, "", "  ")
			differences = append(differences, fmt.Sprintf(
				"%s differs\npackage:\n%s\nhelm:\n%s",
				key,
				packageJSON,
				helmJSON,
			))
		}
	}
	if len(differences) > 0 {
		return fmt.Errorf("manifest parity check failed:\n%s", strings.Join(differences, "\n"))
	}
	return nil
}

// VerifySecurityPosture applies release-gate checks to Helm-rendered objects.
func VerifySecurityPosture(helmManifestPath string) error {
	objects, err := readObjects(helmManifestPath)
	if err != nil {
		return fmt.Errorf("read Helm manifest: %w", err)
	}

	var (
		deploymentFound       bool
		crdFound              bool
		securityPostureErrors []string
	)
	for i := range objects {
		object := &objects[i]
		switch object.GetKind() {
		case "Namespace":
			securityPostureErrors = append(securityPostureErrors, "chart must not render a Namespace")
		case "CustomResourceDefinition":
			crdFound = true
			if object.GetAnnotations()[argocdSyncOptionsAnnotation] != argocdPruneFalse {
				securityPostureErrors = append(
					securityPostureErrors,
					fmt.Sprintf(
						"chart CRD %q is missing the Argo CD Prune=false policy",
						object.GetName(),
					),
				)
			}
		case "Secret":
			if !strings.HasSuffix(object.GetName(), "-osokconfig") {
				securityPostureErrors = append(
					securityPostureErrors,
					fmt.Sprintf("chart must not create credential Secret %q", object.GetName()),
				)
			}
		case "Deployment":
			deploymentFound = true
			var deployment appsv1.Deployment
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &deployment); err != nil {
				return fmt.Errorf("decode Deployment %q: %w", object.GetName(), err)
			}
			securityPostureErrors = append(securityPostureErrors, deploymentSecurityErrors(&deployment)...)
		}
	}
	if !deploymentFound {
		securityPostureErrors = append(securityPostureErrors, "chart did not render a Deployment")
	}
	if !crdFound {
		securityPostureErrors = append(securityPostureErrors, "chart did not render a CustomResourceDefinition")
	}
	if len(securityPostureErrors) > 0 {
		return fmt.Errorf("Helm security checks failed:\n- %s", strings.Join(securityPostureErrors, "\n- "))
	}
	return nil
}

func deploymentSecurityErrors(deployment *appsv1.Deployment) []string {
	var securityErrors []string
	podSpec := deployment.Spec.Template.Spec
	if podSpec.ServiceAccountName == "" || podSpec.ServiceAccountName == "default" {
		securityErrors = append(securityErrors, "Deployment must use a dedicated service account")
	}
	if podSpec.SecurityContext == nil ||
		podSpec.SecurityContext.RunAsNonRoot == nil ||
		!*podSpec.SecurityContext.RunAsNonRoot {
		securityErrors = append(securityErrors, "Deployment must require runAsNonRoot")
	}
	if podSpec.SecurityContext == nil ||
		podSpec.SecurityContext.SeccompProfile == nil ||
		podSpec.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		securityErrors = append(securityErrors, "Deployment must use the RuntimeDefault seccomp profile")
	}
	for _, volume := range podSpec.Volumes {
		if volume.HostPath != nil {
			securityErrors = append(securityErrors, fmt.Sprintf("Deployment volume %q must not use hostPath", volume.Name))
		}
	}
	for _, container := range append(podSpec.InitContainers, podSpec.Containers...) {
		if container.SecurityContext == nil {
			securityErrors = append(securityErrors, fmt.Sprintf("container %q is missing a security context", container.Name))
			continue
		}
		if container.SecurityContext.AllowPrivilegeEscalation == nil ||
			*container.SecurityContext.AllowPrivilegeEscalation {
			securityErrors = append(
				securityErrors,
				fmt.Sprintf("container %q must disable privilege escalation", container.Name),
			)
		}
		if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			securityErrors = append(securityErrors, fmt.Sprintf("container %q must not be privileged", container.Name))
		}
		if container.SecurityContext.Capabilities == nil ||
			!containsCapability(container.SecurityContext.Capabilities.Drop, corev1.Capability("ALL")) {
			securityErrors = append(securityErrors, fmt.Sprintf("container %q must drop ALL capabilities", container.Name))
		}
		if _, err := parseControllerImage(container.Image); err != nil {
			securityErrors = append(securityErrors, fmt.Sprintf("container %q image is not exact: %v", container.Name, err))
		}
	}
	return securityErrors
}

func containsCapability(values []corev1.Capability, expected corev1.Capability) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

// WriteChecksum writes a conventional SHA-256 checksum file next to a release
// artifact.
func WriteChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	outputPath := path + ".sha256"
	output := fmt.Sprintf("%s  %s\n", checksum, filepath.Base(path))
	if err := os.WriteFile(outputPath, []byte(output), 0o644); err != nil {
		return "", err
	}
	return checksum, nil
}

// WriteChartArchive creates a Helm-compatible chart archive with sorted files
// and fixed metadata so identical chart input produces an identical digest.
func WriteChartArchive(chartDir, outputPath string) error {
	chartDir = filepath.Clean(chartDir)
	if chartDir == "." || chartDir == string(filepath.Separator) {
		return fmt.Errorf("refusing to archive unsafe chart directory %q", chartDir)
	}
	info, err := os.Stat(chartDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("chart path %q is not a directory", chartDir)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	var files []string
	if err := filepath.WalkDir(chartDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("chart may not contain symlink %q", path)
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(files)

	tempFile, err := os.CreateTemp(filepath.Dir(outputPath), ".osok-chart-*.tgz")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	gzipWriter, err := gzip.NewWriterLevel(tempFile, gzip.BestCompression)
	if err != nil {
		tempFile.Close()
		return err
	}
	gzipWriter.Header.ModTime = deterministicArchiveTime
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)

	archiveRoot := filepath.Base(chartDir)
	for _, path := range files {
		if err := addArchiveFile(tarWriter, chartDir, archiveRoot, path); err != nil {
			tarWriter.Close()
			gzipWriter.Close()
			tempFile.Close()
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		gzipWriter.Close()
		tempFile.Close()
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, outputPath)
}

func addArchiveFile(writer *tar.Writer, chartDir, archiveRoot, path string) error {
	relativePath, err := filepath.Rel(chartDir, path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(filepath.Join(archiveRoot, relativePath))
	header.Mode = 0o644
	header.Uid = 0
	header.Gid = 0
	header.Uname = ""
	header.Gname = ""
	header.ModTime = deterministicArchiveTime
	header.AccessTime = time.Time{}
	header.ChangeTime = time.Time{}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(writer, file); err != nil {
		return err
	}
	return nil
}

type controllerImage struct {
	Repository string
	Tag        string
	Digest     string
}

func parseControllerImage(value string) (controllerImage, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return controllerImage{}, errors.New("controller image is required")
	}

	if repository, digest, found := strings.Cut(value, "@"); found {
		if repository == "" || !digestPattern.MatchString(digest) {
			return controllerImage{}, fmt.Errorf("invalid digest-pinned controller image %q", value)
		}
		return controllerImage{Repository: repository, Digest: digest}, nil
	}

	lastSlash := strings.LastIndex(value, "/")
	lastColon := strings.LastIndex(value, ":")
	if lastColon <= lastSlash || lastColon == len(value)-1 {
		return controllerImage{}, fmt.Errorf("controller image %q must include a tag or sha256 digest", value)
	}
	if !imageTagPattern.MatchString(value[lastColon+1:]) {
		return controllerImage{}, fmt.Errorf("controller image %q contains an invalid tag", value)
	}
	return controllerImage{
		Repository: value[:lastColon],
		Tag:        value[lastColon+1:],
	}, nil
}

func validateGenerateOptions(opts GenerateOptions) error {
	switch {
	case !groupPattern.MatchString(opts.Group):
		return fmt.Errorf("group %q must be a lowercase DNS label", opts.Group)
	case strings.TrimSpace(opts.Version) == "":
		return errors.New("version is required")
	case !strings.HasPrefix(opts.Version, "v"):
		return fmt.Errorf("version %q must retain the leading v used by the controller image tag", opts.Version)
	case !chartVersionPattern.MatchString(strings.TrimPrefix(opts.Version, "v")):
		return fmt.Errorf("version %q is not a supported semantic version", opts.Version)
	case strings.TrimSpace(opts.ManifestPath) == "":
		return errors.New("package manifest path is required")
	case strings.TrimSpace(opts.SkeletonDir) == "":
		return errors.New("chart skeleton directory is required")
	case strings.TrimSpace(opts.OutputDir) == "":
		return errors.New("chart output directory is required")
	case strings.TrimSpace(opts.CRDOutputPath) == "":
		return errors.New("CRD output path is required")
	}
	expectedChartName := chartName(opts.Group)
	if filepath.Base(filepath.Clean(opts.OutputDir)) != expectedChartName {
		return fmt.Errorf("chart output directory must end in %q", expectedChartName)
	}
	skeletonPath, err := filepath.Abs(opts.SkeletonDir)
	if err != nil {
		return err
	}
	outputPath, err := filepath.Abs(opts.OutputDir)
	if err != nil {
		return err
	}
	if skeletonPath == outputPath {
		return errors.New("chart output directory must differ from the source skeleton")
	}
	if !strings.HasPrefix(filepath.Base(opts.CRDOutputPath), opts.Group+"-crds-") ||
		filepath.Ext(opts.CRDOutputPath) != ".yaml" {
		return fmt.Errorf("CRD output filename must use %s-crds-<version>.yaml", opts.Group)
	}
	return nil
}

func chartName(group string) string {
	return "oci-service-operator-" + group + "-chart"
}

func resetGeneratedDirectory(path string) error {
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return fmt.Errorf("refusing to replace unsafe generated directory %q", path)
	}
	if err := os.RemoveAll(cleaned); err != nil {
		return fmt.Errorf("remove prior generated chart %q: %w", cleaned, err)
	}
	if err := os.MkdirAll(cleaned, 0o755); err != nil {
		return fmt.Errorf("create generated chart directory %q: %w", cleaned, err)
	}
	return nil
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}
		destinationPath := filepath.Join(destination, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(destinationPath, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("chart skeleton may not contain symlink %q", path)
		}
		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer sourceFile.Close()
		destinationFile, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(destinationFile, sourceFile); err != nil {
			destinationFile.Close()
			return err
		}
		return destinationFile.Close()
	})
}

func renderTemplateDirectory(root string, replacements map[string]string) error {
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(paths)
	for _, sourcePath := range paths {
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("read template %q: %w", sourcePath, err)
		}
		rendered := string(content)
		for placeholder, value := range replacements {
			rendered = strings.ReplaceAll(rendered, placeholder, value)
		}
		if strings.Contains(rendered, "__OSOK_") {
			return fmt.Errorf("template %q contains an unresolved OSOK placeholder", sourcePath)
		}
		destinationPath := strings.TrimSuffix(sourcePath, ".tmpl")
		if err := os.WriteFile(destinationPath, []byte(rendered), 0o644); err != nil {
			return fmt.Errorf("write rendered file %q: %w", destinationPath, err)
		}
		if destinationPath != sourcePath {
			if err := os.Remove(sourcePath); err != nil {
				return fmt.Errorf("remove template source %q: %w", sourcePath, err)
			}
		}
	}
	return nil
}

func readObjects(path string) ([]unstructured.Unstructured, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := utilyaml.NewYAMLOrJSONDecoder(file, 4096)
	var objects []unstructured.Unstructured
	for {
		raw := map[string]interface{}{}
		if err := decoder.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if len(raw) == 0 {
			continue
		}
		objects = append(objects, unstructured.Unstructured{Object: raw})
	}
	return objects, nil
}

func generateRBAC(objects []unstructured.Unstructured, path string) error {
	var documents []string
	for i := range objects {
		object := objects[i].DeepCopy()
		if object.GetKind() != "Role" && object.GetKind() != "ClusterRole" {
			continue
		}
		if object.GetNamespace() != "" {
			object.SetNamespace(helmNamespaceToken)
		}
		if err := parameterizeCredentialSecret(object); err != nil {
			return err
		}
		content, err := yaml.Marshal(object.Object)
		if err != nil {
			return fmt.Errorf("marshal %s/%s: %w", object.GetKind(), object.GetName(), err)
		}
		rendered := strings.ReplaceAll(
			string(content),
			"namespace: "+helmNamespaceToken,
			"namespace: {{ .Release.Namespace }}",
		)
		rendered = strings.ReplaceAll(
			rendered,
			"- __OSOK_CREDENTIAL_SECRET__",
			"- {{ .Values.credentials.existingSecret | quote }}",
		)
		documents = append(documents, strings.TrimSpace(rendered))
	}
	if len(documents) == 0 {
		return errors.New("package manifest did not contain Role or ClusterRole resources")
	}
	sort.Strings(documents)
	output := strings.Join(documents, "\n---\n") + "\n"
	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		return fmt.Errorf("write generated RBAC template: %w", err)
	}
	return nil
}

func parameterizeCredentialSecret(object *unstructured.Unstructured) error {
	rules, found, err := unstructured.NestedSlice(object.Object, "rules")
	if err != nil || !found {
		return err
	}
	for i, rawRule := range rules {
		rule, ok := rawRule.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s/%s contains a non-object RBAC rule", object.GetKind(), object.GetName())
		}
		resources, _, err := unstructured.NestedStringSlice(rule, "resources")
		if err != nil {
			return err
		}
		if !containsString(resources, "secrets") {
			continue
		}
		resourceNames, found, err := unstructured.NestedStringSlice(rule, "resourceNames")
		if err != nil {
			return err
		}
		if found && len(resourceNames) == 1 && resourceNames[0] == "ocicredentials" {
			if err := unstructured.SetNestedStringSlice(rule, []string{"__OSOK_CREDENTIAL_SECRET__"}, "resourceNames"); err != nil {
				return err
			}
			rules[i] = rule
		}
	}
	return unstructured.SetNestedSlice(object.Object, rules, "rules")
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func generateCRDs(objects []unstructured.Unstructured, chartCRDDir, companionPath string) error {
	if err := os.MkdirAll(chartCRDDir, 0o755); err != nil {
		return fmt.Errorf("create chart CRD directory: %w", err)
	}
	var companionDocuments []string
	for i := range objects {
		object := objects[i].DeepCopy()
		if object.GetKind() != "CustomResourceDefinition" {
			continue
		}
		annotations := object.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[argocdSyncOptionsAnnotation] = argocdPruneFalse
		object.SetAnnotations(annotations)

		content, err := yaml.Marshal(object.Object)
		if err != nil {
			return fmt.Errorf("marshal CRD %q: %w", object.GetName(), err)
		}
		filename := object.GetName() + ".yaml"
		if err := os.WriteFile(filepath.Join(chartCRDDir, filename), content, 0o644); err != nil {
			return fmt.Errorf("write chart CRD %q: %w", object.GetName(), err)
		}
		companionDocuments = append(companionDocuments, strings.TrimSpace(string(content)))
	}
	if len(companionDocuments) == 0 {
		return errors.New("package manifest did not contain a CustomResourceDefinition")
	}
	sort.Strings(companionDocuments)
	companion := strings.Join(companionDocuments, "\n---\n") + "\n"
	if err := os.MkdirAll(filepath.Dir(companionPath), 0o755); err != nil {
		return fmt.Errorf("create companion CRD directory: %w", err)
	}
	if err := os.WriteFile(companionPath, []byte(companion), 0o644); err != nil {
		return fmt.Errorf("write companion CRD manifest: %w", err)
	}
	if _, err := WriteChecksum(companionPath); err != nil {
		return fmt.Errorf("write companion CRD checksum: %w", err)
	}
	return nil
}

func normalizeObjects(objects []unstructured.Unstructured, packageOutput bool) (map[string]map[string]interface{}, error) {
	normalized := make(map[string]map[string]interface{}, len(objects))
	for i := range objects {
		object := objects[i].DeepCopy()
		if packageOutput && object.GetKind() == "Namespace" {
			continue
		}
		if object.GetKind() == "ServiceAccount" &&
			strings.HasSuffix(object.GetName(), "-controller-manager") {
			continue
		}
		if object.GetKind() == "CustomResourceDefinition" {
			annotations := object.GetAnnotations()
			delete(annotations, argocdSyncOptionsAnnotation)
			if len(annotations) == 0 {
				object.SetAnnotations(nil)
			} else {
				object.SetAnnotations(annotations)
			}
		}
		if object.GetKind() == "ConfigMap" {
			if err := normalizeManagerConfig(object); err != nil {
				return nil, err
			}
		}
		if object.GetKind() == "Deployment" {
			unstructured.RemoveNestedField(object.Object, "spec", "template", "spec", "serviceAccountName")
		}
		if object.GetKind() == "RoleBinding" || object.GetKind() == "ClusterRoleBinding" {
			if err := normalizeServiceAccountSubjects(object); err != nil {
				return nil, err
			}
		}
		key := objectKey(object)
		if _, exists := normalized[key]; exists {
			return nil, fmt.Errorf("duplicate object %s", key)
		}
		normalized[key] = object.Object
	}
	return normalized, nil
}

func normalizeServiceAccountSubjects(object *unstructured.Unstructured) error {
	subjects, found, err := unstructured.NestedSlice(object.Object, "subjects")
	if err != nil || !found {
		return err
	}
	for i, rawSubject := range subjects {
		subject, ok := rawSubject.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s/%s contains a non-object subject", object.GetKind(), object.GetName())
		}
		if subject["kind"] == "ServiceAccount" {
			subject["name"] = serviceAccountToken
			subjects[i] = subject
		}
	}
	return unstructured.SetNestedSlice(object.Object, subjects, "subjects")
}

func normalizeManagerConfig(object *unstructured.Unstructured) error {
	data, found, err := unstructured.NestedStringMap(object.Object, "data")
	if err != nil || !found {
		return err
	}
	config, found := data["controller_manager_config.yaml"]
	if !found {
		return nil
	}
	var parsed interface{}
	if err := yaml.Unmarshal([]byte(config), &parsed); err != nil {
		return fmt.Errorf("parse manager config in ConfigMap %q: %w", object.GetName(), err)
	}
	delete(object.Object, "data")
	object.Object["data"] = map[string]interface{}{
		"controller_manager_config.yaml": parsed,
	}
	return nil
}

func objectKey(object *unstructured.Unstructured) string {
	return fmt.Sprintf(
		"%s|%s|%s|%s",
		object.GetAPIVersion(),
		object.GetKind(),
		object.GetNamespace(),
		object.GetName(),
	)
}
