/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocimock

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/common"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	generatedruntime "github.com/oracle/oci-service-operator/pkg/servicemanager/generatedruntime"
)

// EvidenceCRUDResponder turns one sanitized CRUD recording into a reusable,
// stateful mock. Unlike strict replay, reads are selected by lifecycle phase,
// so extra reconciles do not consume a finite interaction sequence. The real
// OCI SDK still serializes requests and decodes every response.
type EvidenceCRUDResponder struct {
	mu sync.Mutex

	create  evidenceInteraction
	update  *evidenceInteraction
	delete  *evidenceInteraction
	deletes []evidenceInteraction
	reads   map[evidencePhase][]evidenceInteraction
	extra   []evidenceInteraction

	phase        evidencePhase
	operations   map[Operation]int
	phaseReads   map[evidencePhase]int
	readCursors  map[string]int
	deleteCursor int
}

type evidencePhase string

const (
	evidenceBeforeCreate evidencePhase = "before-create"
	evidenceCreated      evidencePhase = "created"
	evidenceUpdated      evidencePhase = "updated"
	evidenceDeleted      evidencePhase = "deleted"
)

type evidenceCassette struct {
	Interactions []evidenceInteraction `yaml:"interactions"`
}

type evidenceInteraction struct {
	Request  evidenceRequest  `yaml:"request"`
	Response evidenceResponse `yaml:"response"`
}

type evidenceRequest struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
	Query  string `yaml:"query"`
	Body   string `yaml:"body"`
}

type evidenceResponse struct {
	StatusCode int                 `yaml:"statusCode"`
	Headers    map[string][]string `yaml:"headers"`
	Body       string              `yaml:"body"`
}

// OpenEvidenceCRUD creates a credential-free SDK session whose base path is
// derived from the recording's primary create route.
func OpenEvidenceCRUD(path string) (*Session, *EvidenceCRUDResponder, error) {
	responder, err := NewEvidenceCRUDResponder(path)
	if err != nil {
		return nil, nil, err
	}
	segments := strings.Split(strings.Trim(responder.create.Request.Path, "/"), "/")
	basePath := ""
	if len(segments) > 1 {
		basePath = segments[0]
	}
	session, err := Open(Options{Host: "https://oci.mock.invalid", BasePath: basePath, Responder: responder})
	if err != nil {
		return nil, nil, err
	}
	return session, responder, nil
}

// InitializeResource gives a typed CR the stable Kubernetes identity used by
// generated retry-token and checkpoint logic.
func InitializeResource(resource metav1.Object, name string) {
	if resource == nil {
		return
	}
	resource.SetName(name)
	resource.SetNamespace("default")
	resource.SetUID(types.UID(name + "-uid"))
}

// RunEvidenceLifecycle applies the recording's create and update fixtures to a
// typed CR while production service-manager code owns all request construction,
// SDK calls, lifecycle decisions, and status projection.
func RunEvidenceLifecycle[T any](resource T, spec any, client LifecycleClient[T], evidence *EvidenceCRUDResponder) error {
	if evidence == nil {
		return errors.New("OCI CRUD evidence responder is required")
	}
	var mutationErr error
	err := RunLifecycle(context.Background(), LifecycleScenario[T]{
		Resource:      resource,
		Client:        client,
		CreateContext: generatedruntime.WithSkipExistingBeforeCreate,
		ValidateCreated: func(current T) error {
			return evidence.ValidateProjection(current)
		},
		Mutate: func(T) {
			mutationErr = evidence.DecodeUpdateSpec(spec)
		},
		ValidateUpdated: func(current T) error {
			if mutationErr != nil {
				return mutationErr
			}
			return evidence.ValidateProjection(current)
		},
		RetryError: func(err error) bool {
			return evidenceHTTPStatus(err, http.StatusNotFound, http.StatusConflict, http.StatusTooManyRequests)
		},
		RetryDeleteError: func(err error) bool {
			return evidenceHTTPStatus(err, http.StatusConflict, http.StatusTooManyRequests)
		},
	})
	if mutationErr != nil {
		return mutationErr
	}
	return err
}

func evidenceHTTPStatus(err error, statuses ...int) bool {
	if err == nil {
		return false
	}
	serviceErr, ok := common.IsServiceError(err)
	for _, status := range statuses {
		if ok && serviceErr.GetHTTPStatusCode() == status {
			return true
		}
		if strings.Contains(strings.ToLower(err.Error()), fmt.Sprintf("http status code: %d", status)) {
			return true
		}
	}
	return false
}

// NewEvidenceCRUDResponder loads recorded request/response evidence and
// classifies its primary CRUD interactions and lifecycle reads.
func NewEvidenceCRUDResponder(path string) (*EvidenceCRUDResponder, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read OCI CRUD evidence: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(false)
	var cassette evidenceCassette
	if err := decoder.Decode(&cassette); err != nil {
		return nil, fmt.Errorf("decode OCI CRUD evidence: %w", err)
	}
	if len(cassette.Interactions) == 0 {
		return nil, errors.New("OCI CRUD evidence contains no interactions")
	}

	createIndex := -1
	updateIndex := -1
	deleteIndex := -1
	var deleteCandidates []int
	for index, interaction := range cassette.Interactions {
		if strings.EqualFold(interaction.Request.Method, http.MethodDelete) {
			deleteCandidates = append(deleteCandidates, index)
		}
	}
	if len(deleteCandidates) > 0 {
		deleteIndex = deleteCandidates[0]
	}
	for index, interaction := range cassette.Interactions {
		if !strings.EqualFold(interaction.Request.Method, http.MethodPost) {
			continue
		}
		if evidenceCreatePathMatchesItem(interaction.Request.Path, cassette.Interactions, -1, deleteIndex) {
			createIndex = index
			break
		}
	}
	if createIndex < 0 {
		for index, interaction := range cassette.Interactions {
			if strings.EqualFold(interaction.Request.Method, http.MethodPost) {
				createIndex = index
				break
			}
		}
	}
	if createIndex < 0 {
		return nil, errors.New("OCI CRUD evidence has no create POST")
	}
	for index := createIndex + 1; index < len(cassette.Interactions); index++ {
		if deleteIndex >= 0 && index >= deleteIndex {
			break
		}
		interaction := cassette.Interactions[index]
		method := strings.ToUpper(interaction.Request.Method)
		if method == http.MethodPut || method == http.MethodPatch ||
			(method == http.MethodPost && deleteIndex >= 0 && interaction.Request.Path == cassette.Interactions[deleteIndex].Request.Path) {
			updateIndex = index
		}
	}

	responder := &EvidenceCRUDResponder{
		create:      cassette.Interactions[createIndex],
		reads:       map[evidencePhase][]evidenceInteraction{},
		phase:       evidenceBeforeCreate,
		operations:  map[Operation]int{},
		phaseReads:  map[evidencePhase]int{},
		readCursors: map[string]int{},
	}
	if updateIndex >= 0 {
		interaction := cassette.Interactions[updateIndex]
		responder.update = &interaction
	}
	if deleteIndex >= 0 {
		interaction := cassette.Interactions[deleteIndex]
		responder.delete = &interaction
		for _, index := range deleteCandidates {
			if cassette.Interactions[index].Request.Path == interaction.Request.Path {
				responder.deletes = append(responder.deletes, cassette.Interactions[index])
			}
		}
	}

	for index, interaction := range cassette.Interactions {
		if strings.ToUpper(interaction.Request.Method) != http.MethodGet {
			if index != createIndex && index != updateIndex && index != deleteIndex {
				responder.extra = append(responder.extra, interaction)
			}
			continue
		}
		phase := classifyEvidenceReadPhase(index, createIndex, updateIndex, deleteIndex)
		responder.reads[phase] = append(responder.reads[phase], interaction)
	}
	if len(responder.reads[evidenceCreated]) == 0 {
		return nil, errors.New("OCI CRUD evidence has no read after create")
	}
	if responder.update != nil {
		if len(responder.reads[evidenceUpdated]) == 0 {
			return nil, errors.New("OCI CRUD evidence has no read after update")
		}
	}
	if responder.delete != nil {
		if len(responder.reads[evidenceDeleted]) == 0 {
			return nil, errors.New("OCI CRUD evidence has no read after delete")
		}
	}
	return responder, nil
}

// DecodeCreateSpec initializes a typed CR spec from the recorded create body.
func (r *EvidenceCRUDResponder) DecodeCreateSpec(target any) error {
	if r == nil {
		return errors.New("nil OCI CRUD evidence responder")
	}
	return decodeEvidenceJSON(r.create.Request.Body, target, "create spec")
}

// DecodeUpdateSpec applies the recorded mutable update fields to a typed CR spec.
func (r *EvidenceCRUDResponder) DecodeUpdateSpec(target any) error {
	if r == nil || r.update == nil {
		return errors.New("OCI CRUD evidence has no update request")
	}
	return decodeEvidenceJSON(r.update.Request.Body, target, "update spec")
}

// ValidateProjection checks the common identity, display name, and lifecycle
// fields that both the recorded OCI response and typed CR status expose.
func (r *EvidenceCRUDResponder) ValidateProjection(resource any) error {
	if r == nil {
		return errors.New("nil OCI CRUD evidence responder")
	}
	r.mu.Lock()
	phase := r.phase
	r.mu.Unlock()
	interactions := r.reads[phase]
	if len(interactions) == 0 {
		return fmt.Errorf("OCI CRUD evidence has no %s projection", phase)
	}
	var lastErr error
	for index := len(interactions) - 1; index >= 0; index-- {
		if err := validateEvidenceProjection(resource, interactions[index].Response.Body); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// Respond implements the mock transport using lifecycle phase rather than a
// one-shot replay cursor.
func (r *EvidenceCRUDResponder) Respond(request Request) (Response, error) {
	if r == nil {
		return Response{}, errors.New("nil OCI CRUD evidence responder")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	switch {
	case evidenceFullRequestMatches(request, r.create.Request):
		r.phase = evidenceCreated
		r.operations[OperationCreate]++
		return evidenceHTTPResponse(r.create.Response), nil
	case r.update != nil && evidenceFullRequestMatches(request, r.update.Request):
		r.phase = evidenceUpdated
		r.operations[OperationUpdate]++
		return evidenceHTTPResponse(r.update.Response), nil
	case r.delete != nil && evidenceRequestMatches(request, r.delete.Request):
		r.phase = evidenceDeleted
		r.operations[OperationDelete]++
		index := r.deleteCursor
		if index >= len(r.deletes) {
			index = len(r.deletes) - 1
		}
		if r.deleteCursor < len(r.deletes) {
			r.deleteCursor++
		}
		return evidenceHTTPResponse(r.deletes[index].Response), nil
	case strings.EqualFold(request.Method, http.MethodGet):
		interactions := r.reads[r.phase]
		var matched *evidenceInteraction
		if r.phase == evidenceDeleted {
			var candidates []evidenceInteraction
			for _, interaction := range interactions {
				if evidenceReadMatches(request, interaction.Request) {
					candidates = append(candidates, interaction)
				}
			}
			if len(candidates) > 0 {
				key := evidenceReadCursorKey(r.phase, request)
				index := r.readCursors[key]
				if index >= len(candidates) {
					index = len(candidates) - 1
				}
				interaction := candidates[index]
				matched = &interaction
				r.readCursors[key]++
			}
		} else {
			for index := len(interactions) - 1; index >= 0; index-- {
				if evidenceReadMatches(request, interactions[index].Request) && successfulStatus(interactions[index].Response.StatusCode) {
					interaction := interactions[index]
					matched = &interaction
					break
				}
			}
		}
		if matched == nil {
			return Response{}, fmt.Errorf("OCI CRUD evidence has no %s read for %s %s", r.phase, request.Method, request.URL.String())
		}
		r.operations[OperationRead]++
		r.phaseReads[r.phase]++
		return evidenceHTTPResponse(matched.Response), nil
	default:
		var driftErr error
		for _, interaction := range r.extra {
			if !evidenceRequestMatches(request, interaction.Request) {
				continue
			}
			if err := compareEvidenceBody(request.Body, interaction.Request.Body); err != nil {
				driftErr = err
				continue
			}
			return evidenceHTTPResponse(interaction.Response), nil
		}
		for _, interaction := range []evidenceInteraction{r.create, evidenceInteractionValue(r.update)} {
			if evidenceRequestMatches(request, interaction.Request) {
				driftErr = compareEvidenceBody(request.Body, interaction.Request.Body)
			}
		}
		if driftErr != nil {
			return Response{}, fmt.Errorf("OCI CRUD evidence request drift for %s %s: %w", request.Method, request.URL.String(), driftErr)
		}
		return Response{}, fmt.Errorf("OCI CRUD evidence has no route for %s %s", request.Method, request.URL.String())
	}
}

func evidenceInteractionValue(interaction *evidenceInteraction) evidenceInteraction {
	if interaction == nil {
		return evidenceInteraction{}
	}
	return *interaction
}

func evidenceCreatePathMatchesItem(collectionPath string, interactions []evidenceInteraction, updateIndex, deleteIndex int) bool {
	collectionPath = strings.TrimSuffix(collectionPath, "/") + "/"
	for _, index := range []int{updateIndex, deleteIndex} {
		if index >= 0 && strings.HasPrefix(interactions[index].Request.Path, collectionPath) {
			return true
		}
	}
	return false
}

func evidenceReadCursorKey(phase evidencePhase, request Request) string {
	return string(phase) + "\x00" + request.URL.Path + "\x00" + normalizedEvidenceQuery(request.URL.Query())
}

// Verify requires every primary lifecycle operation represented by the
// recording to be exercised by the production service manager.
func (r *EvidenceCRUDResponder) Verify() error {
	if r == nil {
		return errors.New("nil OCI CRUD evidence responder")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	required := []Operation{OperationCreate, OperationRead}
	if r.update != nil {
		required = append(required, OperationUpdate)
	}
	if r.delete != nil {
		required = append(required, OperationDelete)
	}
	var missing []string
	for _, operation := range required {
		if r.operations[operation] == 0 {
			missing = append(missing, string(operation))
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("OCI CRUD evidence did not exercise operation(s): %s", strings.Join(missing, ", "))
	}
	for _, phase := range []evidencePhase{evidenceCreated, evidenceUpdated, evidenceDeleted} {
		if len(r.reads[phase]) > 0 && r.phaseReads[phase] == 0 {
			return fmt.Errorf("OCI CRUD evidence did not exercise a %s read", phase)
		}
	}
	return nil
}

func classifyEvidenceReadPhase(index, createIndex, updateIndex, deleteIndex int) evidencePhase {
	switch {
	case index < createIndex:
		return evidenceBeforeCreate
	case deleteIndex >= 0 && index > deleteIndex:
		return evidenceDeleted
	case updateIndex >= 0 && index > updateIndex:
		return evidenceUpdated
	default:
		return evidenceCreated
	}
}

func evidenceRequestMatches(actual Request, expected evidenceRequest) bool {
	if !strings.EqualFold(actual.Method, expected.Method) || actual.URL == nil || actual.URL.Path != expected.Path {
		return false
	}
	return normalizedEvidenceQuery(actual.URL.Query()) == normalizedEvidenceRawQuery(expected.Query)
}

func evidenceFullRequestMatches(actual Request, expected evidenceRequest) bool {
	return evidenceRequestMatches(actual, expected) && compareEvidenceBody(actual.Body, expected.Body) == nil
}

func evidenceReadMatches(actual Request, expected evidenceRequest) bool {
	if actual.URL == nil || actual.URL.Path != expected.Path {
		return false
	}
	// Read identity is carried by the item path. List queries are compared so a
	// missing identity filter cannot be hidden by the mock.
	if expected.Query == "" {
		return true
	}
	return normalizedEvidenceQuery(actual.URL.Query()) == normalizedEvidenceRawQuery(expected.Query)
}

func normalizedEvidenceQuery(values url.Values) string {
	return values.Encode()
}

func normalizedEvidenceRawQuery(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return raw
	}
	return values.Encode()
}

func compareEvidenceBody(actual []byte, expected string) error {
	if strings.TrimSpace(expected) == "" {
		if len(bytes.TrimSpace(actual)) == 0 {
			return nil
		}
		return fmt.Errorf("unexpected body %s", string(actual))
	}
	var actualValue any
	if err := json.Unmarshal(actual, &actualValue); err != nil {
		return fmt.Errorf("decode actual JSON: %w", err)
	}
	var expectedValue any
	if err := json.Unmarshal([]byte(expected), &expectedValue); err != nil {
		return fmt.Errorf("decode expected JSON: %w", err)
	}
	if !reflect.DeepEqual(actualValue, expectedValue) {
		return fmt.Errorf("body mismatch: actual=%s expected=%s", string(actual), expected)
	}
	return nil
}

func evidenceHTTPResponse(recorded evidenceResponse) Response {
	headers := make(http.Header, len(recorded.Headers))
	for name, values := range recorded.Headers {
		headers[name] = append([]string(nil), values...)
	}
	return Response{StatusCode: recorded.StatusCode, Header: headers, Body: []byte(recorded.Body)}
}

func decodeEvidenceJSON(body string, target any, label string) error {
	if target == nil {
		return fmt.Errorf("%s target is nil", label)
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("%s body is empty", label)
	}
	if err := json.Unmarshal([]byte(body), target); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	return nil
}

func validateEvidenceProjection(resource any, responseBody string) error {
	resourceJSON, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("marshal typed resource: %w", err)
	}
	var object map[string]any
	if err := json.Unmarshal(resourceJSON, &object); err != nil {
		return fmt.Errorf("decode typed resource: %w", err)
	}
	status, _ := object["status"].(map[string]any)
	if status == nil {
		return errors.New("typed resource exposes no status object")
	}
	var expected map[string]any
	if err := json.Unmarshal([]byte(responseBody), &expected); err != nil {
		return fmt.Errorf("decode recorded response: %w", err)
	}
	checks := 0
	for _, field := range []string{"id", "displayName", "lifecycleState"} {
		want, exists := expected[field]
		if !exists || want == nil || want == "" {
			continue
		}
		got, exposed := status[field]
		if !exposed && field == "id" {
			if osok, ok := status["status"].(map[string]any); ok {
				got, exposed = osok["ocid"]
			}
		}
		if !exposed {
			continue
		}
		checks++
		if !reflect.DeepEqual(got, want) {
			return fmt.Errorf("status.%s = %#v, want %#v", field, got, want)
		}
	}
	if checks == 0 {
		keys := make([]string, 0, len(expected))
		for key := range expected {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			want := expected[key]
			if !evidenceScalar(want) {
				continue
			}
			got, exposed := status[key]
			if !exposed {
				continue
			}
			checks++
			if !reflect.DeepEqual(got, want) {
				return fmt.Errorf("status.%s = %#v, want %#v", key, got, want)
			}
		}
	}
	if checks == 0 {
		return errors.New("typed resource status exposes no scalar field from the recorded response")
	}
	return nil
}

func evidenceScalar(value any) bool {
	switch value.(type) {
	case string, bool, float64:
		return true
	default:
		return false
	}
}
