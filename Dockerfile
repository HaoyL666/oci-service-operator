#
# Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
# Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
#

# Build the manager binary and cross-compile the target binary.
FROM golang:1.25 AS builder

WORKDIR /workspace
COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY cmd/ cmd/
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/
COPY pkg/ pkg/
COPY go_ensurefips/ go_ensurefips/
COPY main.go ./

ARG CONTROLLER_MAIN=main.go
ARG TARGETOS
ARG TARGETARCH
ARG CGO_ENABLED=1
ARG GOEXPERIMENT=boringcrypto
ENV GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=${CGO_ENABLED} GOEXPERIMENT=${GOEXPERIMENT}
RUN go build -mod vendor -o manager ${CONTROLLER_MAIN}

FROM oraclelinux:9-slim
WORKDIR /

COPY --from=builder /workspace/manager .

ARG SKIP_FIPS=true
ENV OSOK_SKIP_FIPS=${SKIP_FIPS}

USER 65532:65532

ENTRYPOINT ["/manager"]
