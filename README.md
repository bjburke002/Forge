# Forge

An AWS-native internal developer platform on EKS. Teams describe a service in a short
contract; the platform builds, deploys, exposes and observes it, with secure defaults
baked in.

> **Status:** Phases 0–3 complete: AWS foundation, platform components, and a
> hand-built golden path (`examples/payments-api`) running end to end.

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

See the [architecture document](docs/) for the full design and [`docs/adr/`](docs/adr/)
for decisions.

## Repository layout

| Path | Contents |
|---|---|
| `infra/terraform/` | Main root: VPC, EKS, IAM, Karpenter, Argo CD bootstrap |
| `infra/terraform/persistent/` | Resources that survive teardown (ECR template, CI role, certificate, secrets) |
| `infra/terraform/modules/` | One module per foundation layer |
| `charts/platform/` | Argo CD Applications for platform components |
| `charts/<service>/` | Kustomize manifests per service |
| `examples/payments-api/` | Sample Go service used to prove the golden path |
| `cmd/`, `pkg/`, `internal/` | Forge CLI and controller |
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
| Terraform variables (admin role ARN, GitHub repository for CI) | copy each `terraform.tfvars.example` to `terraform.tfvars` and fill in |
| GitHub Actions secrets | `ROLE_ARN`, `AWS_REGION`, `DEPLOY_KEY` (a deploy key with write access) |

The region is `us-east-1` throughout; changing it means updating the same files.

These values are currently repeated across files. Consolidating them into Terraform
variables and, from Phase 4, controller configuration is planned, so that adapting
Forge to another account means editing one place per value.

## Getting started

Apply the persistent root once:

    cd infra/terraform/persistent
    terraform init && terraform apply

Then build and tear down the platform as needed:

    cd infra/terraform
    make up      # build the cluster; Argo CD installs the rest
    make down    # tear it all down

Day-to-day access and troubleshooting: [`docs/operations.md`](docs/operations.md).
