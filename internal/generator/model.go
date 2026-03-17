/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package generator

// PackageModel is the intermediate representation rendered into an OSOK API package.
type PackageModel struct {
	Service       ServiceConfig
	Domain        string
	Version       string
	GroupDNSName  string
	SampleOrder   int
	Resources     []ResourceModel
	Controller    ControllerOutputModel
	PackageOutput PackageOutputModel
}

// PackageOutputModel describes the non-API generated files owned by the generator contract.
type PackageOutputModel struct {
	Generate bool
	Metadata PackageMetadataModel
	Install  InstallKustomizationModel
}

// ControllerOutputModel describes generated controller and manager registration files.
type ControllerOutputModel struct {
	Generate    bool
	Controllers []ControllerFileModel
	Registrar   ServiceRegistrarModel
}

// ControllerFileModel describes one generated controllers/<service>/<resource>_controller.go file.
type ControllerFileModel struct {
	PackageName             string
	GroupDNSName            string
	Version                 string
	APIImportPath           string
	APIImportAlias          string
	Kind                    string
	KindPlural              string
	FileStem                string
	ControllerType          string
	LegacyFieldName         string
	LegacyFieldType         string
	AdditionalRBAC          []RBACRuleModel
	MaxConcurrentReconciles *int
	UseAliasedCoreImport    bool
}

// ServiceRegistrarModel describes one generated pkg/manager/services/<service>.go file.
type ServiceRegistrarModel struct {
	FileStem              string
	RegisterFuncName      string
	APIImportPath         string
	APIImportAlias        string
	NeedsAPIImport        bool
	ControllerImportPath  string
	ControllerImportAlias string
	ManagerImports        []ImportModel
	Resources             []ServiceRegistrarResourceModel
}

// ImportModel describes one Go import and its optional alias.
type ImportModel struct {
	Alias string
	Path  string
}

// ServiceRegistrarResourceModel describes one controller registration block inside a service registrar file.
type ServiceRegistrarResourceModel struct {
	ControllerType            string
	Kind                      string
	ServiceManagerConstructor string
	ControllerLogName         string
	RecorderName              string
	Webhook                   bool
}

// RBACRuleModel renders one additional kubebuilder RBAC marker line.
type RBACRuleModel struct {
	Groups    string
	Resources string
	Verbs     string
}

// PackageMetadataModel renders to packages/<group>/metadata.env.
type PackageMetadataModel struct {
	PackageName            string
	PackageNamespace       string
	PackageNamePrefix      string
	CRDPaths               string
	RBACPaths              string
	DefaultControllerImage string
}

// InstallKustomizationModel renders to packages/<group>/install/kustomization.yaml.
type InstallKustomizationModel struct {
	Namespace  string
	NamePrefix string
	Resources  []string
	Patches    []InstallPatchModel
}

// InstallPatchModel describes one generated kustomize patch entry.
type InstallPatchModel struct {
	Path   string
	Target string
}

// ResourceModel describes one generated top-level kind inside an OSOK API package.
type ResourceModel struct {
	SDKName             string
	Kind                string
	FileStem            string
	KindPlural          string
	Operations          []string
	LeadingComments     []string
	SpecComments        []string
	HelperTypes         []TypeModel
	SpecFields          []FieldModel
	StatusTypeName      string
	StatusComments      []string
	StatusFields        []FieldModel
	PrintColumns        []PrintColumnModel
	ObjectComments      []string
	ListComments        []string
	Sample              SampleModel
	PrimaryDisplayField string
	CompatibilityLocked bool
}

// TypeModel is one helper type emitted into a resource types file.
type TypeModel struct {
	Name     string
	Comments []string
	Fields   []FieldModel
}

// FieldModel is one renderable Go field in a generated spec type.
type FieldModel struct {
	Name     string
	Type     string
	Tag      string
	Comments []string
	Markers  []string
	Embedded bool
}

// PrintColumnModel is one kubebuilder printcolumn marker for a resource.
type PrintColumnModel struct {
	Name        string
	Type        string
	JSONPath    string
	Description string
	Priority    *int
}

// SampleModel renders one sample YAML for a generated resource.
type SampleModel struct {
	Body         string
	FileName     string
	MetadataName string
	Spec         string
}

// HasSpecField reports whether the generated spec exposes a Go field with the given name.
func (r ResourceModel) HasSpecField(name string) bool {
	for _, field := range r.SpecFields {
		if field.Name == name {
			return true
		}
	}
	return false
}
