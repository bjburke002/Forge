# Forge

An AWS-native internal developer platform on EKS. Teams describe a service in a short
contract; the platform builds, deploys, exposes and observes it, with secure defaults
baked in.

> **Status:** Phases 0–3 complete: AWS foundation, platform components, and a
> hand-built golden path (`examples/payments-api`) running end to end. Phase 4 in
> progress: the Forge controller renders every resource a service needs from a
> ForgeService and reports its status. It runs from a workstation against the
> cluster until it is deployed in-cluster.

## What it does

- **Ephemeral by design:** the platform is torn down nightly and rebuilt from code to
  keep costs low; only state that must survive lives in a separate Terraform root.
- **GitOps:** Terraform builds AWS and bootstraps Argo CD; Argo CD installs everything
  else from this repository.
- **Shared ingress:** one ALB for all services, TLS from an ACM wildcard certificate,
  DNS managed by ExternalDNS.
- **Spot-first capacity** via Karpenter, with hard limits as a cost guardrail.
- **No long-lived credentials:** Identity Center for people, GitHub OIDC for CI,
  EKS Pod Identity for workloads, External Secrets for application secrets.

## Architecture

    Terraform ──> VPC · EKS · IAM · Argo CD
                                    │
                                    ▼
               Argo CD (watches this repo) ──> platform components ──> services
                                    ▲
    GitHub Actions ──> ECR ──> image tag commit

See the [architecture document](docs/architecture.md) for the full design and the
[contract decisions](docs/service-contract-decisions.md) for how the service contract was settled.

## Repository layout

| Path | Contents |
|---|---|
| `infra/terraform/` | Main root: VPC, EKS, IAM, Karpenter, Argo CD bootstrap |
| `infra/terraform/persistent/` | Resources that survive teardown (ECR template, CI role, certificate, secrets) |
| `infra/terraform/modules/` | One module per foundation layer |
| `charts/platform/` | Argo CD Applications for platform components |
| `charts/<service>/` | Kustomize manifests per service |
| `examples/payments-api/` | Sample Go service used to prove the golden path |
| `pkg/contract/v1alpha1/` | The ForgeService contract; the CRD is generated from it |
| `internal/controller/` | Reconciler, renderers and status |
| `cmd/forge-controller/` | Controller entrypoint |
| `config/` | Generated CRD and RBAC, and a sample ForgeService |
| `docs/` | Architecture, service checklist, contract decisions |
| `.github/workflows/` | Pull request checks and deploy pipeline |

## Prerequisites

- An AWS account with IAM Identity Center access
- Terraform ≥ 1.11, AWS CLI v2, kubectl, Go, Docker
- A Route 53 hosted zone and an S3 bucket for Terraform state

## Using your own AWS account

The repository is wired to the author's account, domain and GitHub repository.
To run it yourself, replace these values:

| Value | Where |
|---|---|
| AWS account ID and state bucket | `infra/terraform/backend.tf`, `infra/terraform/persistent/backend.tf` |
| AWS account ID (ECR address) | `.github/workflows/deploy.yaml`, `charts/payments/kustomization.yaml` |
| Domain (`forgeappservices.com`) | `infra/terraform/main.tf`, `infra/terraform/persistent/main.tf`, `infra/terraform/modules/iam/main.tf`, `infra/terraform/makefile`, `infra/terraform/modules/argocd/values/argo.values.yaml`, `charts/platform/external-dns.yaml`, `charts/payments/payments-ingress.yaml` |
| GitHub repository URL | `charts/platform/*.yaml` (`repoURL`), `go.mod` |
| Sample service image and host | `config/samples/payments/forgeservice.yaml` |
| Public subnet CIDRs, monitoring namespace | Controller flags (see [Running the controller](#running-the-controller)) |
| Terraform variables (admin role ARN, GitHub repository for CI) | copy each `terraform.tfvars.example` to `terraform.tfvars` and fill in |
| GitHub Actions secrets | `ROLE_ARN`, `AWS_REGION`, `DEPLOY_KEY` (a deploy key with write access) |

The region is `us-east-1` throughout; changing it means updating the same files.

These values are currently repeated across files. Consolidating them into Terraform
variables, and giving the controller a ConfigMap in place of flags, is planned, so
that adapting Forge to another account means editing one place per value.

## Getting started

Apply the persistent root once:

    cd infra/terraform/persistent
    terraform init && terraform apply

Then build and tear down the platform as needed:

    cd infra/terraform
    make up      # build the cluster; Argo CD installs the rest
    make down    # tear it all down

## Running the controller

The CRD and RBAC are applied by hand for now and do not survive teardown, so apply
them after each `make up`:

    kubectl apply -f config/crd/
    kubectl apply -f config/rbac/namespace.yaml
    kubectl apply -f config/rbac/

Run the controller against the cluster, then create a service:

    go run ./cmd/forge-controller \
      --alb-subnet-cidrs=10.20.0.0/24,10.20.1.0/24 \
      --monitoring-namespace=kube-prometheus

    kubectl create namespace forge-test
    kubectl apply -n forge-test -f config/samples/payments/forgeservice.yaml
    kubectl get fsvc -n forge-test

The CIDRs are the public subnets, where the ALB runs. The sample uses the same host as
the hand-written payments service; give it another host if both are running.

After changing the contract types or the RBAC markers, regenerate the CRD, deep-copy
code and ClusterRole:

    go tool controller-gen object crd rbac:roleName=forge-controller paths=./... \
      output:crd:dir=./config/crd output:rbac:dir=./config/rbac
