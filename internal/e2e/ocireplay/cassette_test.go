/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type dispatcherFunc func(*http.Request) (*http.Response, error)

func (f dispatcherFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestRecordSanitizesAndReplays(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recording.yaml")
	delegate := dispatcherFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "ocid1.compartment.oc1..sensitive") {
			t.Fatalf("delegate request body = %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type":   []string{"application/json"},
				"Opc-Request-Id": []string{"live-request-id"},
			},
			Body: io.NopCloser(strings.NewReader(`{"id":"ocid1.budget.oc1..sensitive","password":"do-not-store"}`)),
		}, nil
	})

	recorder, err := Open(Options{Mode: ModeRecord, Path: path, Delegate: delegate})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost,
		"https://usage.us-ashburn-1.oci.oraclecloud.com/20190111/budgets?compartmentId=ocid1.compartment.oc1..sensitive",
		strings.NewReader(`{"compartmentId":"ocid1.compartment.oc1..sensitive","privateKey":"do-not-store"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Signature secret")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Opc-Retry-Token", "random-token")
	response, err := recorder.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(response.Body); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"do-not-store", "Signature secret", "random-token", "ocid1.compartment", "ocid1.budget"} {
		if bytes.Contains(content, []byte(forbidden)) {
			t.Fatalf("recording contains %q:\n%s", forbidden, content)
		}
	}
	for _, expected := range []string{"<redacted>", "<ocid:1>", "<ocid:2>"} {
		if !bytes.Contains(content, []byte(expected)) {
			t.Fatalf("recording does not contain %q:\n%s", expected, content)
		}
	}

	replay, err := Open(Options{Mode: ModeReplay, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	replayRequest, err := http.NewRequest(http.MethodPost,
		"https://usage.us-ashburn-1.oci.oraclecloud.com/20190111/budgets?compartmentId=ocid1.compartment.oc1..different",
		strings.NewReader(`{"privateKey":"another-value","compartmentId":"ocid1.compartment.oc1..different"}`))
	if err != nil {
		t.Fatal(err)
	}
	replayRequest.Header.Set("Authorization", "unrelated")
	replayRequest.Header.Set("Content-Type", "application/json")
	replayRequest.Header.Set("Opc-Retry-Token", "another-random-token")
	replayed, err := replay.Do(replayRequest)
	if err != nil {
		t.Fatal(err)
	}
	replayedBody, err := io.ReadAll(replayed.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(replayedBody), `{"id":"ocid1.replay.oc1..cassette2","password":"<redacted>"}`; got != want {
		t.Fatalf("replayed body = %s, want %s", got, want)
	}
	if err := replay.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReplayMatchesUnusedInteractionsOutOfOrder(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recording.yaml")
	content := `version: 1
interactions:
  - request:
      method: GET
      host: example.test
      path: /first
    response:
      statusCode: 200
      body: first
      encoding: text
  - request:
      method: GET
      host: example.test
      path: /second
    response:
      statusCode: 200
      body: second
      encoding: text
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cassette, err := Open(Options{Mode: ModeReplay, Path: path})
	if err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	errorsFound := make(chan error, 2)
	for _, suffix := range []string{"second", "first"} {
		wait.Add(1)
		go func(suffix string) {
			defer wait.Done()
			request, requestErr := http.NewRequest(http.MethodGet, "https://example.test/"+suffix, nil)
			if requestErr != nil {
				errorsFound <- requestErr
				return
			}
			response, requestErr := cassette.Do(request)
			if requestErr != nil {
				errorsFound <- requestErr
				return
			}
			body, requestErr := io.ReadAll(response.Body)
			if requestErr != nil {
				errorsFound <- requestErr
				return
			}
			if string(body) != suffix {
				errorsFound <- errors.New("unexpected replay body")
			}
		}(suffix)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
	if err := cassette.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReplayReportsMismatchAndUnusedInteractions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recording.yaml")
	content := `version: 1
interactions:
  - request:
      method: GET
      host: example.test
      path: /expected
    response:
      statusCode: 204
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cassette, err := Open(Options{Mode: ModeReplay, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodGet, "https://example.test/unexpected", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cassette.Do(request); err == nil || !strings.Contains(err.Error(), "no unused OCI interaction matches") {
		t.Fatalf("Do() error = %v", err)
	}
	if err := cassette.Close(); err == nil || !strings.Contains(err.Error(), "unused interaction") {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestOpenRejectsUnknownFieldsAndOverwrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recording.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nunknown: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(Options{Mode: ModeReplay, Path: path}); err == nil || !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("Open(replay) error = %v", err)
	}
	if _, err := Open(Options{Mode: ModeRecord, Path: path, Delegate: dispatcherFunc(nil)}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Open(record) error = %v", err)
	}
}

func TestBodyLimitIsEnforced(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "recording.yaml")
	cassette, err := Open(Options{
		Mode:         ModeRecord,
		Path:         path,
		MaxBodyBytes: 3,
		Delegate: dispatcherFunc(func(*http.Request) (*http.Response, error) {
			return nil, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, "https://example.test", strings.NewReader("four"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cassette.Do(request); err == nil || !strings.Contains(err.Error(), "exceeds 3 byte") {
		t.Fatalf("Do() error = %v", err)
	}
}
