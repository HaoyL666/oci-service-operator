# Oracle Core VCN Service

- [Introduction](#introduction)
- [Create Policies](#create-policies)
- [VCN Specification Highlights](#vcn-specification-highlights)
- [Lifecycle and Status](#lifecycle-and-status)
- [Create a VCN](#create-a-vcn)
- [Bind to an Existing VCN](#bind-to-an-existing-vcn)

## Introduction

The Oracle Core Virtual Cloud Network (VCN) service in the OCI Service Operator for Kubernetes (OSOK) lets you create or bind OCI VCNs from Kubernetes custom resources.

The controller supports two primary flows:

- **Create** a new VCN by providing the required network configuration such as `compartmentId` and `cidrBlock` or `cidrBlocks`.
- **Bind** a CR to an existing VCN by supplying the existing OCI VCN OCID in `spec.id`.

## Create Policies

The OCI identity used by OSOK must be allowed to manage VCN resources in the target compartment.

**For Instance Principal**

```plain
Allow dynamic-group <OSOK_DYNAMIC_GROUP> to manage virtual-network-family in compartment <COMPARTMENT_NAME>
```

**For User Principal**

```plain
Allow group <SERVICE_BROKER_GROUP> to manage virtual-network-family in compartment <COMPARTMENT_NAME>
```

## VCN Specification Highlights

The most commonly used fields in the `Vcn` custom resource are:

| Parameter | Description | Type | Required for Create |
| --- | --- | --- | --- |
| `spec.id` | Existing VCN OCID to bind to. | string | No |
| `spec.compartmentId` | Compartment that owns the VCN. | string | Yes |
| `spec.cidrBlock` | Single IPv4 CIDR block for the VCN. Use when creating with one CIDR. | string | Yes, unless `spec.cidrBlocks` is set |
| `spec.cidrBlocks` | One or more IPv4 CIDR blocks for the VCN. Preferred over `spec.cidrBlock` when multiple CIDRs are needed. | array | Yes, unless `spec.cidrBlock` is set |
| `spec.displayName` | Friendly OCI display name for the VCN. | string | Recommended |
| `spec.dnsLabel` | DNS label for the VCN. | string | No |
| `spec.freeformTags` | Free-form OCI tags. | object | No |
| `spec.ipv6PrivateCidrBlocks` | Optional private IPv6 CIDR blocks. | array | No |

When both `spec.cidrBlocks` and `spec.cidrBlock` are present, OSOK sends only `spec.cidrBlocks` to OCI.

## Lifecycle and Status

OSOK updates `status.status.conditions` based on the OCI VCN lifecycle:

- `Provisioning` while OCI reports a non-terminal state such as `PROVISIONING` or `UPDATING`
- `Active` when OCI reports the VCN as `AVAILABLE`
- `Failed` when OCI reports a failed lifecycle state

The controller also records the resolved OCI VCN OCID in `status.status.ocid`.

## Create a VCN

Apply a manifest similar to the sample below to create a new VCN:

```yaml
apiVersion: core.oracle.com/v1beta1
kind: Vcn
metadata:
  name: core-vcn-sample
spec:
  compartmentId: ocid1.compartment.oc1..aaaaaaaaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
  cidrBlock: 10.0.0.0/16
  displayName: osok-core-vcn
  dnsLabel: osokcore
  freeformTags:
    environment: dev
```

```sh
kubectl apply -f <CREATE_YAML>.yaml
```

## Bind to an Existing VCN

To bind a Kubernetes custom resource to an existing OCI VCN, set `spec.id` to the VCN OCID:

```yaml
apiVersion: core.oracle.com/v1beta1
kind: Vcn
metadata:
  name: existing-core-vcn
spec:
  id: ocid1.vcn.oc1.iad.aaaaaaaaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

```sh
kubectl apply -f <BIND_YAML>.yaml
```
