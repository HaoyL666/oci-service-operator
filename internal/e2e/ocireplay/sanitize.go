/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package ocireplay

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ocidPattern = regexp.MustCompile(`ocid1\.[A-Za-z0-9._-]+`)
var ocidPlaceholderPattern = regexp.MustCompile(`<ocid:([0-9]+)>`)

type sanitizer struct {
	ocids map[string]string
}

func newSanitizer() *sanitizer {
	return &sanitizer{ocids: map[string]string{}}
}

func (s *sanitizer) request(req *http.Request, body []byte) (recordedRequest, error) {
	query, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return recordedRequest{}, fmt.Errorf("parse request query: %w", err)
	}
	for key, values := range query {
		for i := range values {
			values[i] = s.text(values[i])
		}
		query[key] = values
	}

	bodyText, encoding, err := s.body(body)
	if err != nil {
		return recordedRequest{}, err
	}

	return recordedRequest{
		Method:   strings.ToUpper(req.Method),
		Host:     strings.ToLower(req.URL.Host),
		Path:     s.text(req.URL.Path),
		Query:    query.Encode(),
		Headers:  s.headers(req.Header, requestHeaderPolicy),
		Body:     bodyText,
		Encoding: encoding,
	}, nil
}

func (s *sanitizer) response(resp *http.Response, body []byte, dispatchErr error) (recordedResponse, error) {
	result := recordedResponse{}
	if resp != nil {
		result.StatusCode = resp.StatusCode
		result.Headers = s.headers(resp.Header, responseHeaderPolicy)
		bodyText, encoding, err := s.body(body)
		if err != nil {
			return recordedResponse{}, err
		}
		result.Body = bodyText
		result.Encoding = encoding
	}
	if dispatchErr != nil {
		result.Error = s.text(dispatchErr.Error())
	}
	return result, nil
}

type headerPolicy map[string]bool

var requestHeaderPolicy = headerPolicy{
	"content-type":    false,
	"if-match":        false,
	"opc-retry-token": true,
}

var responseHeaderPolicy = headerPolicy{
	"content-type":   false,
	"etag":           false,
	"location":       false,
	"opc-next-page":  false,
	"opc-request-id": true,
	"retry-after":    false,
}

func (s *sanitizer) headers(headers http.Header, policy headerPolicy) map[string][]string {
	if len(headers) == 0 {
		return nil
	}
	result := map[string][]string{}
	for name, redact := range policy {
		values, ok := headers[http.CanonicalHeaderKey(name)]
		if !ok {
			continue
		}
		captured := make([]string, len(values))
		for i, value := range values {
			if redact {
				captured[i] = "<redacted>"
			} else {
				captured[i] = s.text(value)
			}
		}
		sort.Strings(captured)
		result[name] = captured
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func (s *sanitizer) body(body []byte) (string, string, error) {
	if len(body) == 0 {
		return "", "", nil
	}

	if json.Valid(body) {
		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			return "", "", fmt.Errorf("decode JSON body: %w", err)
		}
		value = s.jsonValue(value, "")
		var encoded bytes.Buffer
		encoder := json.NewEncoder(&encoded)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(value); err != nil {
			return "", "", fmt.Errorf("encode sanitized JSON body: %w", err)
		}
		return strings.TrimSuffix(encoded.String(), "\n"), "json", nil
	}

	if utf8.Valid(body) {
		return s.text(string(body)), "text", nil
	}
	return base64.StdEncoding.EncodeToString(body), "base64", nil
}

func (s *sanitizer) jsonValue(value any, key string) any {
	if sensitiveJSONKey(key) {
		return "<redacted>"
	}
	switch typed := value.(type) {
	case map[string]any:
		for childKey, child := range typed {
			typed[childKey] = s.jsonValue(child, childKey)
		}
		return typed
	case []any:
		for i := range typed {
			typed[i] = s.jsonValue(typed[i], key)
		}
		return typed
	case string:
		return s.text(typed)
	default:
		return value
	}
}

func sensitiveJSONKey(key string) bool {
	var normalized strings.Builder
	for _, r := range strings.ToLower(key) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			normalized.WriteRune(r)
		}
	}
	value := normalized.String()
	for _, marker := range []string{"authorization", "password", "passphrase", "privatekey", "securitytoken", "secret", "fingerprint"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func (s *sanitizer) text(value string) string {
	return ocidPattern.ReplaceAllStringFunc(value, func(ocid string) string {
		if replacement, ok := s.ocids[ocid]; ok {
			return replacement
		}
		replacement := fmt.Sprintf("<ocid:%d>", len(s.ocids)+1)
		s.ocids[ocid] = replacement
		return replacement
	})
}

func (s *sanitizer) restoreText(value string) string {
	return ocidPlaceholderPattern.ReplaceAllStringFunc(value, func(placeholder string) string {
		for ocid, recordedPlaceholder := range s.ocids {
			if recordedPlaceholder == placeholder {
				return ocid
			}
		}
		matches := ocidPlaceholderPattern.FindStringSubmatch(placeholder)
		generated := "ocid1.replay.oc1..cassette" + matches[1]
		s.ocids[generated] = placeholder
		return generated
	})
}

func decodeBody(body, encoding string) ([]byte, error) {
	switch encoding {
	case "", "json", "text":
		return []byte(body), nil
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return nil, fmt.Errorf("decode base64 cassette body: %w", err)
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("unsupported cassette body encoding %q", encoding)
	}
}
