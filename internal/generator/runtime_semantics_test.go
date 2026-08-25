/*
  Copyright (c) 2026, Oracle and/or its affiliates. All rights reserved.
  Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
*/

package generator

import (
	"testing"

	"github.com/oracle/oci-service-operator/internal/formal"
)

func TestBuildRuntimeSemanticsHonorsRepoAuthoredUpdateOperationSubset(t *testing.T) {
	t.Parallel()

	formalModel := &FormalModel{
		Binding: formal.ControllerBinding{
			Import: formal.ImportModel{
				Operations: formal.Operations{
					Update: []formal.OperationBinding{
						{
							Operation:    "ChangeThingCompartment",
							RequestType:  "ChangeThingCompartmentRequest",
							ResponseType: "ChangeThingCompartmentResponse",
						},
						{
							Operation:    "UpdateThing",
							RequestType:  "UpdateThingRequest",
							ResponseType: "UpdateThingResponse",
						},
					},
				},
			},
		},
		RuntimeLifecycle: &formal.RuntimeLifecycleSpec{
			RepoAuthored: &formal.RuntimeLifecycleRepoAuthoredSemantics{
				Operations: &formal.RuntimeLifecycleOperationSemantics{
					Update: []string{"UpdateThing"},
				},
			},
		},
	}
	runtime := &RuntimeModel{
		Update: &RuntimeOperationModel{MethodName: "UpdateThing"},
	}

	semantics := buildRuntimeSemanticsModel(formalModel, runtime)
	if semantics == nil {
		t.Fatal("buildRuntimeSemanticsModel() = nil")
	}
	if len(semantics.AuxiliaryOperations) != 0 {
		t.Fatalf("AuxiliaryOperations = %#v, want excluded ChangeThingCompartment to be absent", semantics.AuxiliaryOperations)
	}
}
