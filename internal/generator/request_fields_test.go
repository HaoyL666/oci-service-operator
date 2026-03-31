/*
  Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package generator

import (
	"reflect"
	"testing"

	"github.com/oracle/oci-service-operator/internal/ocisdk"
)

func TestRequestLookupPathsUsesPublishedSurfaceAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field ocisdk.Field
		want  []string
	}{
		{
			name: "resource name path uses status then spec name",
			field: ocisdk.Field{
				Name:         "BucketName",
				RequestName:  "bucketName",
				Contribution: ocisdk.FieldContributionPath,
			},
			want: []string{"status.name", "spec.name", "name"},
		},
		{
			name: "namespace path uses status then spec namespace",
			field: ocisdk.Field{
				Name:         "NamespaceName",
				RequestName:  "namespaceName",
				Contribution: ocisdk.FieldContributionPath,
			},
			want: []string{"status.namespace", "spec.namespace", "namespaceName", "namespace"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := requestLookupPaths("Bucket", tt.field)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("requestLookupPaths() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestShouldPreferResourceIDOnlyForIDPathFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		operation      string
		rawName        string
		field          ocisdk.Field
		pathFieldCount int
		want           bool
	}{
		{
			name:      "single id path prefers resource id",
			operation: "Get",
			rawName:   "Bucket",
			field: ocisdk.Field{
				Name:         "BucketId",
				RequestName:  "bucketId",
				Contribution: ocisdk.FieldContributionPath,
			},
			pathFieldCount: 1,
			want:           true,
		},
		{
			name:      "single namespace path does not prefer resource id",
			operation: "List",
			rawName:   "Bucket",
			field: ocisdk.Field{
				Name:         "NamespaceName",
				RequestName:  "namespaceName",
				Contribution: ocisdk.FieldContributionPath,
			},
			pathFieldCount: 1,
			want:           false,
		},
		{
			name:      "multi path non id field does not prefer resource id",
			operation: "Update",
			rawName:   "Bucket",
			field: ocisdk.Field{
				Name:         "BucketName",
				RequestName:  "bucketName",
				Contribution: ocisdk.FieldContributionPath,
			},
			pathFieldCount: 2,
			want:           false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldPreferResourceID(tt.operation, tt.rawName, tt.field, tt.pathFieldCount)
			if got != tt.want {
				t.Fatalf("shouldPreferResourceID() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestRequestFieldsLiteralRendersLookupPaths(t *testing.T) {
	t.Parallel()

	got := requestFieldsLiteral([]RuntimeRequestFieldModel{
		{
			FieldName:    "BucketName",
			RequestName:  "bucketName",
			Contribution: "path",
			LookupPaths:  []string{"status.name", "spec.name", "name"},
		},
	})

	want := `[]generatedruntime.RequestField{{FieldName: "BucketName", RequestName: "bucketName", Contribution: "path", PreferResourceID: false, LookupPaths: []string{"status.name", "spec.name", "name"}}}`
	if got != want {
		t.Fatalf("requestFieldsLiteral() = %q, want %q", got, want)
	}
}
