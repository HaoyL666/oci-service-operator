/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

// Package ocireplay records and replays sanitized OCI HTTP interactions for
// controller integration tests.
package ocireplay

import "net/http"

const (
	// Version is the current on-disk cassette format.
	Version = 1
	// DefaultMaxBodyBytes bounds request and response bodies held in memory.
	DefaultMaxBodyBytes int64 = 4 << 20
)

// Mode controls whether a cassette records live OCI traffic or replays a
// checked-in recording.
type Mode string

const (
	ModeRecord Mode = "record"
	ModeReplay Mode = "replay"
)

// Provenance states how a cassette's OCI interactions were obtained.
type Provenance string

const (
	// ProvenanceRecorded identifies interactions captured from a real OCI API.
	ProvenanceRecorded Provenance = "recorded"
	// ProvenanceSynthetic identifies interactions authored from an OCI SDK/API contract.
	ProvenanceSynthetic Provenance = "synthetic"
)

// Operation names a resource behavior covered by a cassette.
type Operation string

const (
	OperationCreate Operation = "create"
	OperationRead   Operation = "read"
	OperationUpdate Operation = "update"
	OperationDelete Operation = "delete"
	OperationAction Operation = "action"
)

// Metadata describes the scope and provenance of a replay cassette.
type Metadata struct {
	Service    string      `yaml:"service" json:"service"`
	Resource   string      `yaml:"resource" json:"resource"`
	Operations []Operation `yaml:"operations" json:"operations"`
	SDKVersion string      `yaml:"sdkVersion" json:"sdkVersion"`
	Provenance Provenance  `yaml:"provenance" json:"provenance"`
}

// Options configures a cassette.
type Options struct {
	Mode     Mode
	Path     string
	Metadata *Metadata
	Bindings map[string]string
	Delegate interface {
		Do(*http.Request) (*http.Response, error)
	}
	Overwrite    bool
	MaxBodyBytes int64
}

type cassetteFile struct {
	Version      int           `yaml:"version"`
	Metadata     *Metadata     `yaml:"metadata,omitempty"`
	Interactions []interaction `yaml:"interactions"`
}

type interaction struct {
	Request  recordedRequest  `yaml:"request"`
	Response recordedResponse `yaml:"response"`
}

type recordedRequest struct {
	Method   string              `yaml:"method"`
	Host     string              `yaml:"host"`
	Path     string              `yaml:"path"`
	Query    string              `yaml:"query,omitempty"`
	Headers  map[string][]string `yaml:"headers,omitempty"`
	Body     string              `yaml:"body,omitempty"`
	Encoding string              `yaml:"encoding,omitempty"`
}

type recordedResponse struct {
	StatusCode int                 `yaml:"statusCode,omitempty"`
	Headers    map[string][]string `yaml:"headers,omitempty"`
	Body       string              `yaml:"body,omitempty"`
	Encoding   string              `yaml:"encoding,omitempty"`
	Error      string              `yaml:"error,omitempty"`
}
