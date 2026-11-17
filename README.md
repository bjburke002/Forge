# Forge

An AWS-native internal developer platform on EKS. Teams describe a service in a short
contract; the platform builds, deploys, exposes and observes it, with secure defaults
baked in.

## What it does

- **Ephemeral by design:** the platform is torn down nightly and rebuilt from code to
  keep costs low; only state that must survive lives in a separate Terraform root.
- **GitOps:** Terraform builds AWS and bootstraps Argo CD; Argo CD installs the platform
  from this repository and services from forge-gitops.
- **Supply chain:** every image is scanned with Trivy before it is pushed, and signed
  keylessly with cosign alongside a signed SPDX SBOM.
- **Admission policy:** Kyverno admits only images signed by this repository's build
  workflow, keeps the shared ALB to controller-rendered Ingresses with unique hosts on the
  platform domain, and requires probes, requests and a read-only root filesystem; Pod
  Security Admission enforces the restricted profile on every service namespace.
- **SLOs:** availability measured at the load balancer, a 30-day error budget and burn
  rates kept in Amazon Managed Service for Prometheus so they survive teardown, and one
  provisioned Grafana dashboard for every service.
- **Shared ingress:** one ALB for all services, TLS from an ACM wildcard certificate,
  DNS managed by ExternalDNS.
- **A thin CLI:** `forge` validates a ForgeService offline against the CRD, opens the
  pull requests that register and promote a service, and reports Argo CD and SLO state.
  It never writes to the cluster.
- **Spot-first capacity** via Karpenter, with hard limits as a cost guardrail, and
  replicas spread across nodes and availability zones.
- **No long-lived credentials:** Identity Center for people, GitHub OIDC for CI,
  EKS Pod Identity for workloads, External Secrets for application secrets.

## Architecture

    Terraform ──> VPC · EKS · IAM · Argo CD
                                    │
                                    ▼
               Argo CD ──> platform components (this repo, charts/platform)
                       └──> services (forge-gitops) ──> Forge controller ──> Deployment, Ingress, ...
                                    ▲
    GitHub Actions ──> scan, SBOM, sign ──> ECR ──> image tag commit to forge-gitops
    forge create / promote ──────────────────────> pull request on forge-gitops

See the [architecture document](docs/architecture.md) for the full design and the
[contract decisions](docs/service-contract-decisions.md) for how the service contract was settled.

## Repository layout

| Path | Contents |
|---|---|
| `infra/terraform/` | Main root: VPC, EKS, IAM, Karpenter, Argo CD bootstrap |
| `infra/terraform/persistent/` | Resources that survive teardown (ECR template, CI role, certificate, secrets, the AMP workspace and its rules) |
| `infra/terraform/modules/` | One module per foundation layer |
| `charts/platform/` | Argo CD Applications for platform components, the controller, and the services ApplicationSet and AppProject |
| `examples/payments-api/` | Sample Go service used to prove the golden path |
| `pkg/contract/v1alpha1/` | The ForgeService contract; the CRD is generated from it |
| `internal/controller/` | Reconciler, renderers and status |
| `cmd/forge-controller/` | Controller entrypoint |
| `cmd/forge/` | The `forge` CLI's commands |
| `internal/config/`, `internal/validate/`, `internal/gitops/`, `internal/status/`, `internal/slo/` | CLI logic: configuration, CRD validation, GitOps pull requests, cluster status, SLO queries |
| `config/` | Generated CRD (also embedded in the CLI) and RBAC, the controller Deployment and its kustomization, and a sample ForgeService |
| `policies/kyverno/` | Kyverno admission policies, synced by the `kyverno-policies` Application |
| `observability/` | SLI recording rules and the Grafana dashboard (synced by the `observability` Application), and the AMP budget rules (uploaded by Terraform) |
| `docs/` | Architecture, service checklist, contract decisions |
| `.github/workflows/` | Pull request checks, the shared image build, a deploy workflow per image, and the CLI release |

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
| AWS account ID (ECR address) | `config/kustomization.yaml`, the overlays in forge-gitops, `policies/kyverno/registryallowlist.yaml`, `policies/kyverno/signaturecheck.yaml` |
| Domain (`forgeappservices.com`) | `infra/terraform/main.tf`, `infra/terraform/persistent/main.tf`, `infra/terraform/modules/iam/main.tf`, `infra/terraform/makefile`, `infra/terraform/modules/argocd/values/argo.values.yaml`, `charts/platform/external-dns.yaml`, `policies/kyverno/hostdomain.yaml`, ingress hosts in forge-gitops |
| GitHub repository URLs | `charts/platform/*.yaml` (`repoURL`), `go.mod`, the signing identity in `policies/kyverno/signaturecheck.yaml` |
| Sample service image and host | `config/samples/payments/forgeservice.yaml` |
| AMP workspace endpoint (created by the persistent root; `terraform output forge_amp_endpoint`) | `charts/platform/kube-prometheus-stack.yaml` (remote write URL and Grafana data source) |
| Public subnet CIDRs, monitoring namespace | Controller arguments in `config/manager/forge-controller-deployment.yaml` |
| Terraform variables (admin role ARN, GitHub repository for CI) | copy each `terraform.tfvars.example` to `terraform.tfvars` and fill in |
| GitHub Actions secrets | `ROLE_ARN`, `DEPLOY_KEY` (write access to this repository), `GITOPS_DEPLOY_KEY` (write access to forge-gitops) |
| CLI configuration (GitOps repository, domain, AMP endpoint, region) | Your own `config.yaml`; start from `internal/config/config-example.yaml` |

The region is `us-east-1` throughout; changing it means updating the same files.

These values are currently repeated across files. Consolidating them into Terraform
variables, and giving the controller a ConfigMap in place of flags, is planned, so
that adapting Forge to another account means editing one place per value.

## Getting started

Apply the persistent root once:

    cd infra/terraform/persistent
    terraform init && terraform apply

It must exist before `make up`: the main root looks up its Amazon Managed Service for
Prometheus workspace, and the plan fails if it is missing. Re-apply it after changing
`observability/amp-rules/`, which it uploads to AMP.

Then build and tear down the platform as needed:

    cd infra/terraform
    make up      # build the cluster; Argo CD installs the rest
    make down    # tear it all down

## Deploying a service

Argo CD installs the controller, its CRD and RBAC with the rest of the platform. A
service is a folder in forge-gitops (`eks/services/<name>/{base,dev,prod}`); the
ApplicationSet creates an Application per environment, named and namespaced
`<name>-dev` and `<name>-prod`.

1. **Build first.** The service's own repository calls this repository's reusable
   workflow (`bjburke002/Forge/.github/workflows/build-image.yaml@main`), which scans,
   signs and pushes the image to ECR. Kyverno only admits images signed by that
   workflow. A new repository must be added to the CI role's trust policy, and needs a
   write credential for its dev overlay in forge-gitops.
2. **Register.** Write a ForgeService with the pushed image in `spec.image`, check it
   with `forge validate`, then run `forge create` to open a pull request adding its
   `base` and `dev` folders. Merging it brings up `<name>-dev`.
3. **Ship.** From then on, CI writes each new image tag to the dev overlay.
4. **Promote.** `forge promote <name>` opens a pull request copying dev's image to the
   prod overlay, creating it the first time. Merging it deploys `<name>-prod`. To roll
   back, revert that pull request in GitHub.

## The forge CLI

Download a binary for macOS, Linux or Windows from the repository's
[Releases](https://github.com/bjburke002/Forge/releases), with `checksums.txt` to
verify it, or build it yourself:

    go install ./cmd/forge

| Command | Does | Needs |
|---|---|---|
| `forge validate <file>` | Checks a ForgeService against the CRD's schema and CEL rules | Nothing: offline |
| `forge create <file>` | Validates it and opens a pull request adding `eks/services/<name>/base` and `dev` | GitHub token |
| `forge promote <name>` | Opens a pull request copying dev's image to the prod overlay | GitHub token |
| `forge status <name>` | Argo CD sync and health, readiness, replicas, image and URL for dev and prod | Cluster access |
| `forge slo <name>` | 30-day availability, budget left and 1h and 6h burn rates from AMP | AWS credentials |
| `forge config view` | Prints the resolved configuration | Nothing |

`create` and `promote` take `--dry-run` to print the files instead of opening a pull
request. `create` takes `--target` (default `eks`); `promote` finds the service's
backend itself, and a `--target` given to it must match. Only `eks` is supported so far. Every command takes `--config <path>` and `--context <kube context>`.

**Configuration.** The CLI reads `~/.config/forge/config.yaml` if it exists, otherwise
`config.yaml` in a `forge` folder under the operating system's configuration directory,
or the file given with `--config`. Copy `internal/config/config-example.yaml` and fill
in the GitOps repository (`owner/name`), its branch, the platform domain, the Argo CD
namespace, and the AMP endpoint and region. Each setting can also be set with an
environment variable (`FORGE_GITOPS_REPO`, `FORGE_DOMAIN`, `FORGE_AMP_ENDPOINT` and so
on); flags win over environment variables, which win over the file.

**GitHub token.** `create` and `promote` use `FORGE_GITHUB_TOKEN`, then `GITHUB_TOKEN`,
then `gh auth token` if the GitHub CLI is signed in. A fine-grained token limited to
forge-gitops with read and write access to contents and pull requests is enough. The
token is never stored in the configuration file.

**Releases.** Pushing a `v*` tag builds the binaries and publishes a release:

    git tag -a v0.1.0 -m "forge CLI v0.1.0"
    git push origin v0.1.0

## Running the controller locally

For development, stop the in-cluster controller first so two controllers don't fight
over the same objects. Disable auto-sync on the `forge-controller` Application (or
self-heal restores the replica within moments), then:

    kubectl scale deployment forge-controller-deployment -n forge-system --replicas=0
    go run ./cmd/forge-controller \
      --alb-subnet-cidrs=10.20.0.0/24,10.20.1.0/24 \
      --monitoring-namespace=kube-prometheus

Re-enable auto-sync when you're done; Argo CD restores the in-cluster controller.

After changing the contract types or the RBAC markers, regenerate the CRD, deep-copy
code and ClusterRole:

    go tool controller-gen object crd rbac:roleName=forge-controller paths=./... \
      output:crd:dir=./config/crd output:rbac:dir=./config/rbac

## Running the tests

The controller tests include envtest suites, which start a local kube-apiserver and
etcd. `setup-envtest` downloads those binaries once and prints where they are:

    export KUBEBUILDER_ASSETS=$(go run sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.24 use 1.36 -p path)
    go test ./...

Without `KUBEBUILDER_ASSETS`, the `internal/controller` package fails at startup,
because its tests share one envtest environment. The renderer tests need no API server
and run on their own with `go test ./internal/controller/resources/`.
