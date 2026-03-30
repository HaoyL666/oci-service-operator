#
# Copyright (c) 2021, Oracle and/or its affiliates. All rights reserved.
# Licensed under the Universal Permissive License v 1.0 as shown at http://oss.oracle.com/licenses/upl.
#

# Build the manager binary on the native builder platform and cross-compile the target binary.
ARG BUILDPLATFORM
ARG TARGETPLATFORM
FROM --platform=$BUILDPLATFORM golang:1.25 AS builder

WORKDIR /workspace
COPY go.mod go.sum ./
COPY vendor/ vendor/
COPY . ./

ARG CONTROLLER_MAIN=main.go
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG CGO_ENABLED=1
ARG GOEXPERIMENT=boringcrypto
ENV GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=${CGO_ENABLED} GOEXPERIMENT=${GOEXPERIMENT}
RUN go build -mod vendor -a -o manager ${CONTROLLER_MAIN}

FROM oraclelinux:9-slim
WORKDIR /

COPY --from=builder /workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
