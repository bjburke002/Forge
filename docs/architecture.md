# Forge: An AWS-Native Internal Developer Platform

*Architecture, Design Decisions, and Build Strategy*

Revision 2.3  ·  October 2026  
Supersedes Revision 1 (Forge — AWS Developer Platform Architecture)  
Delivery window: 13 weeks  ·  Cost ceiling: \$50/month

## Contents

- [1. Executive Summary](#1-executive-summary)
- [2. What Changed From Revision 1](#2-what-changed-from-revision-1)
- [3. Non-Goals](#3-non-goals)
- [4. Constraints](#4-constraints)
  - [4.1 Time](#41-time)
  - [4.2 Cost](#42-cost)
- [5. Core Technology Decisions](#5-core-technology-decisions)
- [6. Why Terraform Modules](#6-why-terraform-modules)
- [7. Ownership Boundaries](#7-ownership-boundaries)
- [8. High-Level Architecture](#8-high-level-architecture)
- [9. The Forge Service Contract](#9-the-forge-service-contract)
- [10. AWS Foundation](#10-aws-foundation)
  - [10.1 VPC](#101-vpc)
  - [10.2 EKS](#102-eks)
  - [10.3 Karpenter](#103-karpenter)
  - [10.4 Accounts and Access](#104-accounts-and-access)
- [11. Networking and Ingress](#11-networking-and-ingress)
- [12. DNS and TLS](#12-dns-and-tls)
- [13. Workload Identity and Secrets](#13-workload-identity-and-secrets)
- [14. GitOps and the Deployment Path](#14-gitops-and-the-deployment-path)
- [15. CI/CD and Supply Chain](#15-cicd-and-supply-chain)
- [16. Kubernetes Security and Policy](#16-kubernetes-security-and-policy)
  - [16.1 Rendered by the controller](#161-rendered-by-the-controller)
  - [16.2 Enforced by Kyverno](#162-enforced-by-kyverno)
- [17. Observability](#17-observability)
- [18. SLOs and Error Budgets](#18-slos-and-error-budgets)
  - [18.1 The SLI](#181-the-sli)
  - [18.2 The error budget](#182-the-error-budget)
  - [18.3 Burn-rate alerting](#183-burn-rate-alerting)
- [19. Environment Model](#19-environment-model)
- [20. Reliability and Failure Testing](#20-reliability-and-failure-testing)
- [21. Repository Structure](#21-repository-structure)
- [22. Cost Model](#22-cost-model)
  - [22.1 Hourly burn while the cluster is up](#221-hourly-burn-while-the-cluster-is-up)
  - [22.2 Residue that survives teardown](#222-residue-that-survives-teardown)
  - [22.3 Monthly total by uptime discipline](#223-monthly-total-by-uptime-discipline)
- [23. Definition of Done](#23-definition-of-done)
- [24. Open Questions and Deferred Decisions](#24-open-questions-and-deferred-decisions)
- [25. Design Principles in Brief](#25-design-principles-in-brief)
- [26. Build Strategy](#26-build-strategy)

## 1. Executive Summary

Forge is an internal developer platform that turns a declarative service definition into a standardized, secure, observable production workload on Amazon EKS. A developer describes what their service needs; Forge decides how AWS and Kubernetes deliver it.

The platform contract is deliberately independent of cloud-specific implementation detail. Developers write against the Forge Service contract; the platform translates it into AWS and Kubernetes primitives. That indirection is the product — everything else is implementation.

The platform is Go end to end: Go for the platform control plane and CLI, and a Go Kubernetes controller as the reconciliation engine. Terraform modules provision the AWS infrastructure. Helm packages platform components. Argo CD owns deployment state.

> **The central bet.** One Go type definition — the Forge Service contract — is compiled into the CRD schema, consumed by the controller, and used by the CLI for validation. Changing the contract in one place changes every layer that handles workloads, and the compiler enforces it. The AWS foundation sits deliberately outside that boundary and is built with Terraform modules (§6). This is the central design decision.

## 2. What Changed From Revision 1

Revision 1 was architecturally sound but carried three unresolved conflicts and no constraints. This revision resolves them. Read this section first if you are comparing the two documents.

| Area | Revision 1 | Revision 2 | Why |
|---|---|---|---|
| Deployment path | CLI and REST API expose an imperative "deploy" verb, alongside a GitOps section warning against imperative deployment. | Forge writes desired state to Git. Argo CD is the only thing that touches the cluster. No imperative deploy path exists. | Rev 1 contradicted itself. An imperative deploy API is the anti-pattern its own GitOps section warned about. |
| Application rendering | A Helm chart renders app resources (Phase 3); a controller renders the same resources (Phase 8). Overlap unresolved. | The controller renders application resources. Helm is scoped to platform components only. | Two implementations of one job. Building both wastes the scarcest resource in the project, which is time. |
| Availability SLI | Kubernetes readiness as the availability signal, with HTTP success rate explicitly rejected. | Request success rate at the load balancer is primary. Readiness is a secondary internal signal. | A pod can be Ready and serving 500s. Readiness is also sampled at scrape interval, which is too coarse for a 43-minute budget. |
| Control plane | A bespoke REST API (Phase 7), then a CRD and controller that supersede it (Phase 8). | The Kubernetes API server is the control plane. No bespoke REST service. | The API server already provides auth, RBAC, watch, admission, and versioning. Rev 1 built a REST layer and then moved behavior out of it. |
| Service contract | Two incompatible schemas, both labelled forge.dev/v1. | One canonical Go type. CRD schema is generated from it. | The contract is the product. It cannot have two definitions. |
| Constraints | None stated. | Explicit time budget, cost ceiling, non-goals, and cut lines. | Without a time budget and cost ceiling there is no basis for prioritization or cut decisions. |
| Environments | Not addressed. | Explicit single-cluster, namespace-per-environment model with a documented promotion path. | Promotion between environments is core IDP territory and its absence is conspicuous. |

Everything else from Revision 1 — the technology selections, the ownership boundaries, the AWS foundation, the supply-chain design — survives essentially unchanged. Those parts were right. Revision 2.1 (October 2026) moved the AWS foundation from Pulumi with Go to Terraform modules.

Revision 2.2 (October 2026) records the decisions made while building Phases 0 to 2. None of them changes the shape of the architecture; they settle details the earlier revisions left open:

- Forge runs in its own AWS account inside an Organization, with service control policies and IAM Identity Center sign-in (§10.4).
- The region is us-east-1 rather than us-east-2. List prices are identical for everything in §22.
- Terraform is split into two roots by lifecycle: a persistent root for what must survive teardown, and a main root that make up and make down cycle (§7, §21).
- ECR repositories are created on first push from a Terraform-managed template, so onboarding a service needs no infrastructure change (§15).
- Every AWS-calling component, including the VPC CNI, uses EKS Pod Identity; the node role only joins the cluster and pulls images (§13).
- Terraform installs Argo CD, which reads platform components from this repository; service desired state stays in a separate GitOps repository (§14).
- Teardown is scripted: AWS resources created by Kubernetes controllers are removed before terraform destroy, and a leftover check runs after it (§22).
- Two extensions are recorded rather than built: a second runtime on ECS with Fargate, and golden paths for languages other than Go (§24).

Revision 2.3 (October 2026) records the rest of Phase 2 and the Phase 3 golden path, built by hand around a sample service before any controller exists:

- A reference service, payments-api, now runs end to end and proves every resource the controller will render, from the Deployment to the NetworkPolicy (§9, §16.1).
- Rollouts are zero-downtime: a preStop sleep plus ALB readiness gates (§11).
- ExternalDNS runs in sync mode, and teardown waits for it to remove its records (§12).
- External Secrets is implemented, with secret values that never enter Terraform state (§13).
- Argo CD sync waves are ordered by capacity, the monitoring CRDs install from their own Application, cert-manager issues in-cluster webhook certificates, and charts follow a set of rules for running under Argo CD (§14).
- Karpenter nodes have a memory floor, and teardown checks for every resource type that has been left behind (§10.3, §22).
- CI/CD is built: required pull request checks, and a deploy workflow that commits the new image tag. Service manifests live in this repository until the GitOps repository arrives in Phase 5 (§15).
- The VPC CNI enforces NetworkPolicies (§10.2, §16.1).

## 3. Non-Goals

Stating what Forge is not is the cheapest defense against scope creep. Forge does not attempt:

- Multi-cloud portability. The contract is cloud-independent by design, but only one implementation exists. A second AWS runtime (ECS on Fargate) is recorded as a possible future extension in §24; a second cloud is not planned.
- Stateful workloads. No databases, no StatefulSets, no storage lifecycle management. Applications consume managed AWS data services.
- A service mesh. Ingress and NetworkPolicy cover the requirements. mTLS between services is out of scope.
- Hard multi-tenancy. Namespace isolation and NetworkPolicy assume cooperating internal teams, not hostile tenants.
- A web developer portal. The contract, the CLI, and Grafana are the interfaces. Backstage is a plausible future addition, not a requirement.
- Progressive delivery. Rolling updates only. Canary and blue/green are a natural extension point but not part of this build.
- Cost showback per service. Useful, but outside the cost and time budget.

## 4. Constraints

These two constraints shape every decision that follows, and they are stated up front so that trade-offs can be judged against them.

### 4.1 Time

The delivery window is thirteen weeks with approximately 130 hours of engineering capacity. The full plan is estimated at 146 hours, so it carries explicit cut lines rather than optimism. The accompanying project plan identifies a 103-hour must-ship spine, which leaves roughly 27 hours for 38 hours of second-tier work: most of it is expected to land, not all, and the documented cut order is used rather than improvised decisions under pressure.

### 4.2 Cost

A ceiling of \$50 per month. This constraint is more binding than it looks, and it drives a real architectural decision:

> **The EKS control plane costs \$0.10 per hour.** Left running continuously that is \$73 per month — 146% of the entire budget before a single node, NAT gateway, or load balancer exists. Teardown is therefore not a cost optimization. It is a load-bearing architectural requirement, and the infrastructure code must support it from day one.

Section 22 works the numbers. The short version: the full stack costs about \$0.24 per hour while running and about \$5 per month in residue that survives teardown, which puts the ceiling at roughly 190 cluster-hours per month. That is around 44 hours per week of uptime — comfortable for the expected usage pattern, and impossible without teardown discipline.

The useful consequence is that "stands up from nothing and tears down cleanly" becomes a tested property of the platform rather than an aspiration. A cluster that has been hand-patched since March proves nothing about the infrastructure code. A cluster rebuilt from zero every week proves the code is complete.

## 5. Core Technology Decisions

| Concern | Technology | Rationale |
|---|---|---|
| Platform and CLI | Go | Natural fit for Kubernetes controllers, CLIs, and cloud SDKs. Shares the contract type with everything else. |
| Infrastructure as code | Terraform modules | Industry standard for AWS. Local modules map onto the foundation's layers, and destroy/apply round-trips cleanly for teardown. See §6. |
| Cloud | AWS | EKS, ECR, IAM, Route 53, ACM, CloudWatch. |
| Container platform | Amazon EKS | Managed control plane on Kubernetes 1.36, upgrade policy STANDARD. See §10.2 and §22 on extended-support pricing. |
| Node provisioning | Karpenter | v1.15. Spot-first capacity provisioned against actual pod requirements, with hard CPU and memory limits. |
| Platform packaging | Helm | Platform components only, installed by Argo CD from charts/platform. Not used for application workloads. |
| Application rendering | Forge controller (Go) | Reconciles the ForgeService CRD into Kubernetes resources. |
| Deployment | Argo CD | Sole writer to the cluster. ApplicationSet generates per-service Applications. |
| Ingress | AWS Load Balancer Controller | Shared ALB across services via IngressGroup. |
| In-cluster certificates | cert-manager | v1.21. Issues admission webhook certificates inside the cluster; public TLS stays on ACM. |
| DNS and TLS | Route 53 + ExternalDNS + ACM | Automated records; ACM certificates terminated at the ALB. |
| Workload identity | EKS Pod Identity | Short-lived least-privilege credentials. No static keys in pods. |
| Registry | Amazon ECR | Repositories created on first push from a template: immutable tags, scanning, lifecycle rules. |
| Policy | Kyverno | Admission enforcement including cosign signature verification. |
| Observability | Prometheus + Grafana | OpenTelemetry tracing deferred — see §24. |
| CI | GitHub Actions + OIDC | No long-lived AWS credentials anywhere. |
| Supply chain | Trivy + Syft + cosign | Scan, SBOM, keyless signing bound to the workflow identity. |
| Accounts and sign-in | AWS Organizations + IAM Identity Center | Dedicated Forge account with SCP guardrails. Short-lived SSO credentials; no IAM users or access keys. |

## 6. Why Terraform Modules

Revision 2 chose Pulumi with Go so that the infrastructure code could share the contract type too. This revision drops that in favor of Terraform, and the cost should be stated plainly rather than argued away: the AWS foundation no longer type-checks against the contract.

In practice that coupling was thin. The contract describes workloads, and the AWS foundation — VPC, EKS, IAM, ECR, Route 53 — barely depends on its shape. The one per-service AWS concern, IAM access for a service that calls AWS, is defined in Terraform against the ServiceAccount the controller renders (§13), not derived from the contract. The contract is still defined once and still shared by every layer that handles workloads:

```text
// pkg/contract/v1alpha1/types.go — defined once
type ForgeServiceSpec struct {
    Owner         string            `json:"owner"`
    Image         string            `json:"image"`
    Runtime       RuntimeSpec       `json:"runtime"`
    Env           []EnvVar          `json:"env,omitempty"` // Forge's own name/value type, not corev1.EnvVar
    Scaling       ScalingSpec       `json:"scaling"`
    Resources     ResourceSpec      `json:"resources"`
    Ingress       *IngressSpec      `json:"ingress,omitempty"` // absent: no Ingress is rendered
    Health        HealthSpec        `json:"health"`
    Observability ObservabilitySpec `json:"observability"`
    SLO           *SLOSpec          `json:"slo,omitempty"`
}

// consumed by, and type-checked against:
//   controller-gen  -> config/crd/forge.dev_forgeservices.yaml
//   internal/controller -> reconciliation logic
//   (infra/terraform sits deliberately outside this boundary)
//   cmd/forge       -> CLI scaffolding and validation
```

Add a field to the contract and the compiler still tells you every place in the controller and CLI that must handle it. An ordinary service needs no infrastructure change either: ECR repositories are created on first push from a template (§15). Only a service that calls AWS needs a Terraform change, for its IAM role (§13). What is lost is only the compile-time link between the contract and the AWS foundation, which in practice had little to check.

What it brings: Terraform is the more common enterprise choice, its AWS provider and the community terraform-aws-modules (VPC, EKS, Karpenter) are the most heavily trodden path to this exact stack, and terraform plan gives a reviewable diff before every apply. Local modules under infra/terraform/modules map one-to-one onto the foundation's layers. They are called from two roots split by lifecycle (§7), so the teardown round trip in §4 is a single terraform destroy and terraform apply of the main root.

## 7. Ownership Boundaries

Forge must not become one large Terraform configuration that deploys every application. Each layer owns a distinct slice of state, and the boundaries are what keep the system comprehensible.

| Layer | Owns | Explicitly does not own |
|---|---|---|
| Terraform | AWS infrastructure, in two roots. Persistent: ECR repository template and scanning, GitHub OIDC provider and CI role, EC2 Spot service-linked role, ACM certificate. Main: VPC, EKS and add-ons, IAM and Pod Identity roles, Karpenter's interruption queue, and the Argo CD bootstrap. | Anything inside the cluster beyond bootstrapping Argo CD. The state bucket and hosted zone, which are created by hand and looked up. |
| Helm | Platform component packaging: Karpenter, LB Controller, ExternalDNS, Kyverno, kube-prometheus-stack, deployed by Argo CD from charts/platform. | Application workloads. |
| Argo CD | Reconciliation of desired state from Git into the cluster. Sole write path. | Deciding what the desired state should be. |
| Forge controller | Translating a ForgeService CR into Deployment, Service, Ingress, HPA, PDB, ServiceAccount, NetworkPolicy, ServiceMonitor. | AWS resource provisioning. |
| Forge CLI | Developer ergonomics: scaffolding, validation, status aggregation, SLO reporting. | Writing to the cluster. Ever. |

> **The invariant.** Argo CD is the only component with write access to application state in the cluster. CI writes to Git. The CLI writes to Git. The controller writes only resources it owns, in response to a CR that Argo placed. If any other component acquires cluster write credentials, the design has broken.

**Two lifecycles.** Terraform is split into two roots by how long their resources live. The persistent root holds what must survive make down: what CI depends on between sessions (the GitHub OIDC provider, the CI role, the ECR repository template), account-wide settings (registry scanning, the Spot service-linked role) and the ACM certificate. It is applied by hand, rarely, and never destroyed by automation. The main root holds everything make up and make down cycle. Because ECR repositories are created by pushes rather than by Terraform, images survive teardown without being in either root's state.

## 8. High-Level Architecture

```text
  DEVELOPER
      |
      | forge create / git push
      v
  GIT  (service repo + GitOps repo)
      |                                    +--------------------+
      |                                    |  Terraform modules |
      +--> GitHub Actions                  |   (out of band)    |
      |      test -> scan -> build         |                    |
      |      SBOM -> sign -> ECR           |   VPC   IAM   ECR  |
      |      bump image tag in GitOps      |   EKS   Argo  ACM  |
      |                                    +---------+----------+
      v                                              |
  GitOps repo (desired state)                        v
      |                                       AWS FOUNDATION
      v                                              |
  Argo CD  --- ApplicationSet ---> ForgeService CR   |
      |                                 |            |
      |                                 v            |
      |                        FORGE CONTROLLER (Go) |
      |                                 |            |
      |                                 v            |
      +---------------------------> EKS CLUSTER <----+
                                        |
       +--------------+-----------------+---------------+
       |              |                 |               |
       v              v                 v               v
   Deployment      Ingress          Kyverno        ServiceMonitor
   Service         + HPA/PDB        (admission)          |
   ServiceAccount  + NetworkPolicy                       v
       |              |                              Prometheus
       |              v                                  |
       |         Shared ALB --> Route 53 / ACM           v
       |                                              Grafana
       v                                                 |
   EKS Pod Identity --> IAM Role --> AWS services        v
                                                    SLO + Error Budget
```

Two independent provisioning paths meet at the cluster. Terraform builds and destroys the AWS foundation out of band, on the operator's schedule. Argo CD continuously reconciles everything inside the cluster from Git. They intersect only at cluster bootstrap, where Terraform installs Argo CD and creates one root Application pointing at charts/platform in this repository (§14).

## 9. The Forge Service Contract

The contract is the platform. It describes what an application needs, never how AWS or Kubernetes should provide it. There is exactly one definition, expressed as a Kubernetes custom resource and generated from the Go type in §6.

```yaml
apiVersion: forge.dev/v1alpha1
kind: ForgeService
metadata:
  name: payments
  namespace: payments-prod
spec:
  owner: payments-team          # routed to for alerts; required
  image: <account>.dkr.ecr.us-east-1.amazonaws.com/forge/payments-api:sha-a1b2c3d

  runtime:
    port: 8080

  env:                          # plain values only; no secrets, no valueFrom
    - name: LOG_LEVEL
      value: info

  scaling:
    minReplicas: 2
    maxReplicas: 10
    targetCPUUtilization: 70

  resources:
    requests: { cpu: 250m, memory: 256Mi }
    limits:   { memory: 512Mi }     # memory only; there is no CPU limit field

  ingress:                      # omit the block for a service with no Ingress
    host: payments.example.com  # always HTTPS; ACM cert found by host match
    path: /

  health:
    readiness: /health/ready
    liveness:  /health/live

  observability:                # required: every service exposes Prometheus metrics
    metricsPath: /metrics       # default /metrics
    metricsPort: 9090           # separate from runtime.port; never exposed

  slo:
    availability: "99.9"        # percent, 30d rolling
    latencyP99Ms: 500

status:
  conditions: [...]             # Ready, Progressing, Degraded
  observedGeneration: 4
  availableReplicas: 3
  url: https://payments.example.com
  errorBudget:
    remainingMinutes: 31.7
    consumedPercent: 26.6
```

**Note the version.** This is v1alpha1, not v1. The contract will change repeatedly during the build, and shipping a v1 that immediately breaks is worse than labelling its maturity accurately. Promote to v1beta1 once the schema survives two months without a breaking change, and only then plan a conversion webhook.

**Note the kind.** The kind is ForgeService, not Service, so it never collides with the core Kubernetes Service: kubectl get service always resolves to the core kind, which would leave developers typing services.forge.dev. The CRD registers the short name fsvc, so kubectl get fsvc lists them. In prose, "a Forge Service" still means one of these resources.

**Note env.** Environment variables are plain name and value pairs, so configuration such as a log level or a dependency URL can differ between the dev and prod overlays without rebuilding the image. The field uses Forge's own type rather than the Kubernetes EnvVar, which would allow valueFrom references to arbitrary Secrets and ConfigMaps. Values are stored in Git, so secrets never go here; secret references through External Secrets are a later, additive change.

**Note what is absent.** No mention of ALB, Route 53, node types, availability zones, or IAM trust policies. A developer cannot express an AWS-specific concept through this contract, which is what makes the abstraction real rather than decorative.

**Note what is not here: AWS access.** The contract has no field for AWS permissions. Granting them means creating an IAM role and a Pod Identity association, which are AWS resources, and the controller only ever writes Kubernetes objects (§7). A service that calls AWS gets its role from Terraform instead, bound to the ServiceAccount the controller renders, <svc>-sa in the service's namespace; that name is the whole interface between the two. Because AWS permissions are a privilege-escalation path, routing every grant through a reviewed Terraform change is the approval gate this needs anyway. Design: one file per service that needs AWS (infra/terraform/services/<svc>.yaml, listing its namespaces and policy statements), read by a service-identity module that creates the role, an inline policy and one Pod Identity association per namespace. To settle when it is built: a permissions boundary on every service role, and one role per environment rather than per service.

**Reference implementation.** Phase 3 built one service, payments-api, by hand to find out exactly what the controller must produce. The service itself sets the bar every golden-path service meets: liveness that checks nothing but the process, readiness that reflects dependencies, metrics on a separate port that the ALB never exposes, request metrics labelled by matched route with health checks excluded, graceful shutdown on SIGTERM, JSON logs, and a distroless non-root image. What it needed from Kubernetes, split into what comes from the contract and what the platform fixes:

| Resource | From the contract | Fixed by the platform |
|---|---|---|
| Deployment | image, runtime.port, env, resources, health paths | Non-root UID, read-only root filesystem, all capabilities dropped, seccomp RuntimeDefault. No replicas field (the HPA owns it). preStop sleep of 15 s, which with the app's 10 s shutdown fits the 30 s grace period. |
| Service | runtime.port, observability.metricsPort | Named ports http and metrics; the app label the ServiceMonitor selects on. |
| Ingress | ingress.host, ingress.path | forge-shared group, target type ip, HTTP to HTTPS redirect, ALB health check on the readiness path. Never routes the metrics port. |
| Horizontal Pod Autoscaler | scaling.minReplicas, maxReplicas, targetCPUUtilization | Utilization is measured against the CPU request. |
| Pod Disruption Budget | - | maxUnavailable 1 with unhealthyPodEvictionPolicy AlwaysAllow: drains, consolidation and spot replacement take one pod at a time, a single-replica service can still be drained, and a pod that is not Ready never blocks a drain. |
| ServiceAccount | - | One per service, named <svc>-sa, token automount off. No image pull secrets: nodes pull from ECR with the node role. Its name is the stable identity Terraform binds a service's IAM role to (§13). |
| NetworkPolicy | Declared dependencies (future) | Ingress only: the http port from the public subnets (where the ALB lives), the metrics port from the Prometheus namespace. |
| ServiceMonitor | observability.metricsPath, metricsPort | 15 s scrape interval, 10 s timeout. |
| Namespace and Application | - | Readiness-gate injection label on the namespace; Argo CD sync wave after the monitoring stack, with retry and CreateNamespace. |

The right-hand column is the platform's opinion; it never appears in a developer's Service. The middle column is the contract. Anything that ends up in neither is a gap to close before Phase 4.

## 10. AWS Foundation

### 10.1 VPC

- Two availability zones. Three is the production answer; two is the right answer under a \$50 ceiling, and the difference is one Terraform variable.
- Private subnets for all worker capacity; public subnets for the shared ALB only.
- A single NAT gateway, not one per AZ. This is a deliberate, documented availability compromise: an AZ failure taking out the NAT gateway breaks egress for the whole cluster. At \$0.045/hr per gateway with teardown discipline the saving is modest, but the simplicity is real and the trade-off is recorded.
- Subnet tagging for discovery: kubernetes.io/role/elb on public subnets, kubernetes.io/role/internal-elb and karpenter.sh/discovery on private subnets, and karpenter.sh/discovery on the cluster security group EKS creates. Getting these tags wrong is the single most common cause of a controller that silently provisions nothing.
- VPC endpoints deliberately omitted. Interface endpoints for ECR would cut NAT data-processing charges, but at \$0.01/hr each they cost more than they save at this scale. Correct at production volume, wrong here — a good example of a decision that inverts with scale.
- Addressing: a /16 (10.20.0.0/16) with /24 public subnets and /20 private subnets, leaving room for a third AZ. Pods take VPC addresses directly and the VPC CNI uses prefix delegation, so the private subnets are sized for pod density rather than node count.
- The default security group is managed by Terraform with no rules, so nothing can come to rely on it.

### 10.2 EKS

A managed control plane on Kubernetes 1.36, plus a small managed node group to host the components that cannot depend on Karpenter — Karpenter itself, CoreDNS, and the Argo CD controllers. Two t3a.small on-demand instances is sufficient and keeps the bootstrap path free of circular dependencies.

Cluster add-ons: VPC CNI (with prefix delegation, which raises the pod limit on a t3a.small from about 11 to 110), CoreDNS, kube-proxy, the EKS Pod Identity Agent, the EBS CSI driver, and metrics-server, which HPAs need. Add-ons that call AWS receive their permissions through Pod Identity, not the node role. Everything else arrives through Argo CD. The VPC CNI is also configured to enforce NetworkPolicies (enableNetworkPolicy), which adds a policy agent container to every aws-node pod. Without it, Kubernetes accepts NetworkPolicies and silently ignores them.

> **Version currency is a cost issue.** EKS extended support costs \$0.60 per hour — six times the standard rate. A cluster left on an out-of-support version costs \$438/month at 24/7 rather than \$73. Keep the cluster on a supported version. The cluster's upgrade policy is set to STANDARD, so at the end of standard support EKS upgrades the control plane rather than silently moving it to extended-support pricing.

**Cluster access.** Authentication uses EKS access entries (API mode), not the aws-auth ConfigMap. The Identity Center administrator role has an explicit cluster-admin access entry in Terraform, and bootstrap creator permissions are off, so access never depends on who ran the apply. The public API endpoint is restricted to an allowlist of operator IP addresses.

**Managed node headroom.** The two t3a.small nodes exist only for what must run before Karpenter: Karpenter, the Argo CD controllers and CoreDNS. Their memory requests sit around 80%, and small per-node additions such as the network policy agent are enough to push other pods off them. Move the node group to t3a.medium when requests approach 90%, and keep other platform components on Karpenter's nodes.

### 10.3 Karpenter

Karpenter provisions EC2 capacity in response to unschedulable pods, which keeps node sizing a platform concern rather than an application-team concern. The NodePool is spot-first with on-demand fallback, consolidates empty or underused nodes after two minutes, and replaces nodes after 24 hours.

```text
Pending pod
    |
    v
Karpenter NodePool
    |
    +-- requirements: amd64, spot then on-demand, t/m/c, 2-4 vCPU, >= 4 GiB memory
    +-- limits: cpu 20, memory 20Gi   (hard ceiling; cost guardrail)
    +-- disruption: consolidateAfter 2m, expireAfter 24h
    |
    v
EC2NodeClass --> pinned AL2023 AMI, discovery by tag
    |
    v
EC2 spot capacity
```

**Implementation caveat.** If Karpenter runs with hostNetwork enabled, EKS Pod Identity requires the IAM trust policy to grant both sts:AssumeRole and sts:TagSession to the pods.eks.amazonaws.com principal. This differs from IRSA, which often works without the tagging permission, and it produces an opaque AccessDeniedException when missing. Budget an hour for this specific problem.

**Cost guardrail.** The NodePool cpu and memory limits are the last line of defense against a runaway HPA quietly provisioning \$200 of spot capacity overnight. Set them low and deliberately.

**Supporting AWS pieces.** Karpenter v1.15 runs with the controller policy from Karpenter's own template for that version, kept in the repository as a versioned file. Its nodes run as a separate node role with an EC2_LINUX access entry, without which they launch but never join. An SQS queue fed by five EventBridge rules delivers spot interruption warnings, rebalance recommendations, instance state changes, scheduled maintenance and capacity reservation warnings, so nodes are drained before AWS reclaims them. The EC2 Spot service-linked role is created once in the persistent root, since a new account has none.

**Two settings that are easy to miss.** The NodePool only allows amd64 instances, because CI builds amd64 images; otherwise Karpenter picks Graviton whenever it is cheaper and services fail to start. And the EC2NodeClass sets maxPods to 110, because Karpenter does not detect the VPC CNI's prefix delegation and would otherwise size nodes for about 11 pods.

**Memory floor.** The NodePool also requires at least 4 GiB of memory per instance (instance-memory greater than 3071 MiB). Without it, Karpenter picked micro and small instances because they are cheapest per pod; on a 1 GiB node only about 515Mi was allocatable after the kubelet and DaemonSets, and Grafana never became Ready there. The floor matches the cost model's assumption of t3a.medium-class nodes and costs roughly \$1 to \$3 a month at expected usage; in return, bootstrap converges without manual intervention.

### 10.4 Accounts and Access

Forge runs in a dedicated AWS account inside an AWS Organization. The management account holds billing and IAM Identity Center and runs no workloads. Other projects live in their own accounts, so a mistaken destroy or an over-broad policy in Forge cannot reach them, and the budget alert measures Forge alone.

- Service control policies on the workload OU deny leaving the Organization and deny API calls outside us-east-1, with exemptions for global services. They cannot apply to the management account, which is why Forge is a member account.
- People sign in through IAM Identity Center and receive short-lived credentials. There are no IAM users and no long-lived access keys anywhere in the project.
- CI authenticates through GitHub OIDC (§15); workloads and controllers through EKS Pod Identity (§13).
- Cost tracking: a Project tag on every Terraform-managed resource, activated as a cost allocation tag, and an account-level AWS Budgets alert at \$35.

Separate accounts per environment is the production answer (§19) and would double the EKS and NAT cost. One workload account is the budget-constrained version of the same idea.

## 11. Networking and Ingress

The AWS Load Balancer Controller replaces the Traefik-centric approach from the previous architecture. Kubernetes Ingress remains the developer-facing abstraction while the controller provisions and configures AWS Application Load Balancers.

Critically, all Forge services share a single ALB through the IngressGroup annotation. One ALB per Ingress is the default behavior, and it is both a cost problem — \$0.0225/hr each — and a recognized anti-pattern at scale.

```text
# rendered by the controller onto every service Ingress
annotations:
  alb.ingress.kubernetes.io/group.name: forge-shared
  alb.ingress.kubernetes.io/scheme: internet-facing
  alb.ingress.kubernetes.io/target-type: ip
  alb.ingress.kubernetes.io/listen-ports: '[{"HTTPS":443},{"HTTP":80}]'
  alb.ingress.kubernetes.io/ssl-redirect: "443"
  alb.ingress.kubernetes.io/healthcheck-path: /health/ready

Developer                              one ALB, many services
    |                                        |
    v                                        v
ForgeService CR --> Forge controller --> Ingress (group: forge-shared)
                                             |
                                             v
                                  AWS Load Balancer Controller
                                             |
                                             v
                               Shared ALB --> target group --> pod IPs
```

Target type ip routes directly to pod IPs rather than through NodePort, which removes a hop and makes ALB-level metrics attributable to individual pods. That attribution is what makes the SLI in §18 possible.

**Controller configuration.** The controller (v3.6.0) runs one replica and finds the VPC by its Name tag rather than its ID, so the configuration in Git stays valid across rebuilds that recreate the VPC. Setting the region and VPC explicitly also keeps it independent of instance metadata, which Karpenter's nodes block from pods by default.

**A shared ALB is a shared risk.** Any namespace can join the forge-shared group, so anyone allowed to create an Ingress could add rules to the shared ALB, including rules for another service's hostname. A Kyverno policy restricts the group to Ingresses rendered by the Forge controller, and a second rejects any Ingress whose host is already used in the group (§16.2). One host per service is also why Ingresses carry no group.order: rules from different services can never match the same request, so their order on the listener has no effect.

**Zero-downtime rollouts.** Two settings close the gaps between Kubernetes and the ALB. A preStop sleep (15 s) keeps a terminating pod serving until the ALB has deregistered it; without it, rollouts produce occasional 502s. Readiness gates, injected by the Load Balancer Controller into pods in namespaces labelled elbv2.k8s.aws/pod-readiness-gate-inject, stop Kubernetes counting a new pod as Ready until the ALB reports its target healthy; without them, a rollout can briefly leave the ALB with no healthy targets. With both, a request loop across repeated rollouts sees no errors.

**Webhook certificates.** The controller's admission webhooks need a serving certificate that the API server trusts. The chart can generate one, but only at render time, and under Argo CD every render produces a new CA (§14); when the certificate and the webhook configurations were applied from different renders, admission calls failed with x509 errors and Ingresses could not be created. cert-manager now issues the certificate inside the cluster (enableCertManager): the chart renders a self-signed root, a CA issuer and a serving Certificate, and cert-manager's CA injector writes the matching caBundle into both webhook configurations. Nothing certificate-related is generated at render time, so no ignoreDifferences rules are needed. The Service mutator webhook is disabled: Forge exposes services through Ingress, not LoadBalancer Services, and the webhook would otherwise sit in the path of every Service creation in the cluster, including during bootstrap before the controller is ready.

## 12. DNS and TLS

- Route 53 holds the hosted zone and is the DNS authority. \$0.50/month, and the one component that cannot be torn down without losing the zone's name servers. It is created by hand in the Forge account and looked up by Terraform, never managed by it.
- ExternalDNS reconciles Ingress host fields into Route 53 alias records. It runs in sync mode, so records are deleted when their Ingress is, with an owner ID (forge) recorded in TXT records so it only ever touches its own entries, a domain filter on the Forge zone, and event-driven updates rather than a one-minute poll. Alias queries to an ALB are not billed, so query volume is free at this scale.
- ACM issues a wildcard certificate for the zone, validated by DNS. The LB Controller discovers it by host match, so no per-service certificate management exists. It lives in the persistent root, so a rebuild never waits for validation.
- A developer sets ingress.host in the contract. Nothing else: HTTPS, with a redirect from HTTP, is always on and there is no field to turn it off. No certificate request, no DNS record, no annotation.

Use a wildcard certificate rather than per-service certificates. It is one ACM resource, it survives teardown, and it removes an entire class of first-deploy latency where a service waits on certificate validation.

**Where TLS ends.** TLS terminates at the ALB, which can only use ACM certificates. cert-manager is not used for public TLS: the certificates it issues live in Kubernetes Secrets that an ALB cannot read. Its only role is the in-cluster webhook certificates (§11). Traffic from the ALB to pods is plain HTTP inside the VPC, consistent with the service-mesh non-goal in §3; re-encryption to the pod or a mesh would be the answer if that requirement changed.

**Records and teardown.** Teardown deletes Ingresses, then waits until ExternalDNS has removed its records (no TXT ownership records left in the zone) before terraform destroy removes its permissions. If records do survive, the next build adopts them through the same owner ID. Resolvers cache "does not exist" answers for the zone's SOA minimum TTL, so after a rebuild a hostname can stay unresolvable for some minutes; lowering that TTL to 60 s shortens the gap.

## 13. Workload Identity and Secrets

```text
Kubernetes ServiceAccount   (rendered by the controller)
          |
          v
  EKS Pod Identity Association   (Terraform)
          |
          v
     IAM Role  (least privilege, defined in Terraform per service)
          |
          v
     AWS service (S3, SQS, Secrets Manager, ...)
```

No application ever receives a long-lived AWS credential. Pod Identity is preferred over IRSA: association is a cluster-level API call rather than an annotation coupled to the cluster OIDC issuer URL, which means roles survive cluster teardown and rebuild — directly relevant given §4.

**Platform components too.** Pod Identity is used for every AWS-calling component, not only applications. The VPC CNI, the EBS CSI driver, the AWS Load Balancer Controller, ExternalDNS (limited to changing records in the Forge zone) and Karpenter each have their own role. The node role keeps only what a node itself needs: joining the cluster, pulling images, and Session Manager access. Otherwise any pod could borrow the node's credentials and act with every controller's permissions.

For application secrets, use the External Secrets Operator to sync from AWS Secrets Manager into Kubernetes Secrets. The alternative, the Secrets Store CSI driver, mounts secrets as files and avoids Kubernetes Secret objects entirely, which is arguably the stronger security posture. ESO is chosen here because it works with unmodified application code expecting environment variables, and because it is the more common pattern in the field.

> **Naming the tradeoff.** ESO materializes secrets as Kubernetes Secret objects, which are base64-encoded, not encrypted, and readable by anyone with get access in the namespace. Enable EKS envelope encryption with KMS for etcd, and keep RBAC on Secrets tight. The choice trades some security purity for compatibility with unmodified applications, and that trade-off is deliberate.

**As built.** ESO is installed by Argo CD and reads Secrets Manager through its own Pod Identity role, limited to secrets under forge/. A single ClusterSecretStore points at Secrets Manager and is restricted to the namespaces that need it. Secret values are created in the persistent Terraform root from an ephemeral random_password written to a write-only attribute, so they never appear in Terraform state, and they survive teardown so credentials stay stable across rebuilds. Each ExternalSecret lives in the same Argo CD application as the workload that consumes it; Grafana's comes from kube-prometheus-stack's extraManifests.

**Not from Terraform.** Creating the Kubernetes Secret directly from Terraform was considered and rejected: it would put Terraform inside the cluster, breaking the ownership boundary in §7. Terraform creates the AWS side only.

## 14. GitOps and the Deployment Path

This section resolves the central contradiction in Revision 1. There is no imperative deploy path in Forge. CI publishes artifacts. Git holds desired state. Argo CD reconciles. Nothing else writes to the cluster.

```text
forge create payments
        |
        +--> scaffolds service repo (Dockerfile, health endpoints, CI workflow)
        +--> opens a PR against the GitOps repo adding a ForgeService CR
                    |
                    v (human merges)
              GitOps repo
                    |
                    v
        Argo CD ApplicationSet (git directory generator)
                    |
                    +--> Application: payments-dev
                    +--> Application: payments-prod
                    |
                    v
              ForgeService CR applied to cluster
                    |
                    v
              Forge controller reconciles
                    |
                    v
        Deployment / Service / Ingress / HPA / PDB / NetPol / SM
```

| Command | What it actually does |
|---|---|
| forge create &lt;name&gt; | Scaffolds the service repository and opens a PR adding a ForgeService CR to the GitOps repo. Writes to Git, never to the cluster. |
| forge status &lt;name&gt; | Reads the ForgeService CR status subresource and the Argo Application sync state. Read-only. |
| forge promote &lt;name&gt; --to prod | Opens a PR copying the image tag from the dev overlay to the prod overlay. A human merges it. |
| forge rollback &lt;name&gt; | Opens a PR reverting the GitOps commit that changed the image tag. Argo reconciles the revert. |
| forge logs &lt;name&gt; | Streams pod logs via the Kubernetes API. Read-only convenience. |
| forge slo &lt;name&gt; | Queries Prometheus for availability, error budget consumed and remaining, and current burn rate. |

> **The design rule.** Every Forge command that changes the system produces a Git commit. Every command that does not produce a Git commit is read-only. There is no third category. This is a single sentence, it is enforceable by inspection, and it is the strongest thing this document says about deployment.

The ApplicationSet is the glue Revision 1 left unspecified. A git directory generator watching the GitOps repository turns each directory into an Argo Application automatically, so registering a new service requires no Argo configuration — the PR that adds the ForgeService CR is the entire registration process.

**Two repositories, eventually.** Argo CD reads from this repository. Platform components are Applications in charts/platform: Terraform creates one root Application pointing there, and each component appears as its own Application with its own sync status and history. Until Phase 5, service manifests also live here, as Kustomize folders under charts/&lt;service&gt;. The separate GitOps repository and its ApplicationSet replace them in Phase 5, which moves CI's frequent image-tag commits out of the platform repository's history.

**Bootstrap details.** Terraform installs Argo CD with the argo-cd Helm chart and creates the root Application with the argocd-apps chart. Argo CD has no ingress and is reached by port-forward, so its UI is only as exposed as the Kubernetes API. Child Applications are ordered with sync waves (below), which requires restoring Argo CD's health check for Application resources. The root Application deliberately has no deletion finalizer: teardown pauses Argo CD before terraform destroy, and a finalizer nobody processes would hang the destroy.

**Capacity first.** The root Application syncs each wave and waits for it to be healthy before starting the next, and it creates later Applications only when it reaches their wave. Anything that provides capacity must therefore come before anything that might need it; only Karpenter has to fit on the managed nodes. With External Secrets placed ahead of Karpenter, ESO's pods stopped fitting on the managed nodes, its wave never became healthy, and Karpenter never received a NodePool: a deadlock that only appears once the managed nodes are full. The order now is:

| Wave | Applications | Depends on |
|---|---|---|
| -3 | karpenter | Only what Terraform created: IAM, Pod Identity, the interruption queue |
| -2 | karpenter-configs (NodePool, EC2NodeClass) | Karpenter |
| -1 | external-secrets, cert-manager | Capacity from Karpenter |
| 0 | external-secrets-configs (ClusterSecretStore), kube-prometheus-crds, LB Controller, ExternalDNS | ESO's CRDs; cert-manager for the LB Controller's certificate |
| 1 | kube-prometheus-stack | The ClusterSecretStore and the monitoring CRDs |
| 2 | Services (payments) | The ServiceMonitor CRD |

**Charts under Argo CD.** Argo CD renders charts with helm template on every sync and never runs helm install, which breaks two common chart features. Values generated during rendering, such as Grafana's default admin password, change on every render, so the application never settles and self-heal turns that into a rollout loop. Helm hooks, such as kube-prometheus-stack's admission webhook Jobs, can finish without Argo CD noticing and stall the sync. The rules: secrets come from outside the chart via existingSecret and ESO; hooks are disabled where the chart allows; charts with large CRDs use ServerSideApply; certificates are issued by cert-manager inside the cluster rather than generated at render time (§11); and no Application gets auto-sync until it has stayed Synced across several refreshes.

**Monitoring CRDs.** A chart that ships CRDs together with resources of those types can fail its first sync on a fresh cluster. The monitoring CRDs therefore install from their own Application at wave 0, using the prometheus-operator-crds chart, and kube-prometheus-stack runs with crds.enabled set to false. The two chart versions are pinned as a pair that carries the same operator version (prometheus-operator-crds 32.0.1 and kube-prometheus-stack 92.11.0, both operator v0.94.1) and are upgraded together. The CRD Application is annotated Prune=false and Delete=false, because deleting a CRD deletes every resource of that type.

**Expected transient states.** During bootstrap, some Applications briefly report Progressing or Degraded: pods stay Pending while Karpenter launches nodes, and the Prometheus resource reports unavailable until the operator has created its StatefulSet. These clear without intervention. A state that persists after the nodes have joined is a real fault and is investigated as one.

**Controller sizing.** The application controller caches every managed resource, and large CRDs make syncs expensive. It runs with 512Mi requested, a 1Gi limit and GOMEMLIMIT set to 900MiB; without that setting, Go's garbage collector let memory overshoot the limit during the bootstrap rush and the controller was OOM-killed partway through a sync.

## 15. CI/CD and Supply Chain

```text
Pull request
    |
    +-- unit tests
    +-- go vet / staticcheck
    +-- SAST (CodeQL)
    +-- dependency scan (govulncheck)
    |
    v  merge to main
    |
Container build (distroless base, non-root, multi-stage)
    |
    +-- Trivy scan            --> fail on HIGH/CRITICAL with a fix available
    +-- Syft SBOM             --> SPDX, attached as an attestation
    +-- cosign sign           --> keyless, GitHub OIDC identity, Rekor
    |
    v
Amazon ECR  (created on push: immutable tags, scanning, lifecycle)
    |
    v
GitOps repo: bump image tag in the dev overlay, commit
    |
    v
Argo CD reconciles --> dev
    |
    v
forge promote --to prod  --> PR --> human merge --> prod
```

GitHub Actions authenticates to AWS through the OIDC provider and a role whose trust policy accepts only the main branch of this repository. The subject is matched in GitHub's immutable form, with the owner's and repository's numeric IDs (repo:owner@id/repo@id:ref:refs/heads/main), so a deleted and recreated repository with the same name cannot assume the role. The role can log in to ECR and push to forge/\* repositories and nothing else: it cannot delete images, change repository settings, or touch the cluster. No AWS access keys exist in GitHub secrets. The provider and role live in the persistent Terraform root, so CI keeps working while the platform is torn down.

**Repositories on demand.** ECR repositories are not listed in Terraform. A repository creation template in the persistent root covers everything under forge/, and ECR creates a repository the first time CI pushes to a new name, applying immutable tags, encryption and the lifecycle rules (untagged images expire after seven days; the last 20 build images are kept). Scan on push is a registry-level rule for forge/\*. Onboarding a service therefore needs no infrastructure change, and teardown never touches the images.

**As built.** Pull requests run Go vet and tests, a Docker build, and terraform fmt and validate for both roots; a ruleset makes both jobs required on main. That workflow has no path filter on purpose: a required check from a workflow that never starts stays pending and blocks the merge. The deploy workflow does use a path filter, running on pushes to main that touch the service or the workflow itself: it tests, builds and pushes the image tagged with the commit SHA, then commits the new tag to the service's Kustomize folder with kustomize edit set image in its full name=repository:tag form (the short form drops the repository and sends the cluster to Docker Hub). Deploys run one at a time through a concurrency group. Argo CD rolls the change out from there.

**Pushing past branch protection.** The tag commit goes straight to main, so it must bypass the ruleset. GitHub's built-in workflow token cannot be added to a ruleset's bypass list on a personal repository, so the commit is pushed with a deploy key that can. Unlike the workflow token, deploy-key pushes trigger workflows, which is why the deploy path filter must never include the manifests it edits. The scanning, SBOM and signing steps in the diagram are not built yet, and in Phase 5 the tag commit moves to the GitOps repository, which needs its own cross-repository credential.

**Keyless signing is the detail to get right.** cosign with GitHub OIDC binds the signature to the workflow identity — repository, ref, and workflow file — recorded in the Rekor transparency log. Kyverno then verifies against that identity rather than a public key. The guarantee is specific and strong: this image was built by this workflow, from this repository, on this branch, and nothing else can produce a signature that satisfies the policy. That is materially stronger than signing with a shared key.

Fail the build only on HIGH and CRITICAL findings that have a fix available. Failing on unfixable findings trains people to bypass the gate, which is worse than not having one.

## 16. Kubernetes Security and Policy

Baseline controls are rendered by the controller, so they are not optional and cannot be forgotten. Kyverno then enforces at admission, which catches anything applied outside the controller.

### 16.1 Rendered by the controller

- Namespace per service per environment.
- Default-deny ingress NetworkPolicy: the application port only from the public subnets, where the ALB's interfaces live (not the whole VPC, since pods also take VPC addresses), and the metrics port only from the Prometheus namespace. Egress rules, which must then explicitly allow DNS, come later.
- ServiceAccount per service with token automount off. AWS access comes from a Pod Identity association, not a ServiceAccount annotation.
- runAsNonRoot, readOnlyRootFilesystem, all capabilities dropped, seccomp RuntimeDefault.
- A writable emptyDir at /tmp alongside the read-only root filesystem, so runtimes that need scratch space (.NET, Java) work without weakening the policy.
- CPU and memory requests and a memory limit from the contract — required fields, so they cannot be omitted. There is no CPU limit: it throttles a service even when the node has idle CPU, while requests already guarantee each service its share.
- PodDisruptionBudget with maxUnavailable 1, which never blocks a drain whatever the replica count, and an HPA that owns the replica count: the Deployment sets no replicas, or Argo CD's self-heal fights the HPA.
- Readiness and liveness probes from the contract — also required.
- A preStop sleep and readiness-gate injection for zero-downtime rollouts (§11).
- A topology spread constraint across nodes (maxSkew 1, ScheduleAnyway), so replicas land on different nodes where possible and one spot interruption cannot take every pod. Preferred over pod anti-affinity, which Karpenter satisfies by launching a node per replica.

### 16.2 Enforced by Kyverno

- verifyImages against the cosign keyless identity. This is the highest-value policy in the set and should be written first.
- Registry allowlist restricted to the project ECR account.
- Require resource requests, a memory limit, probes, and non-root — defense in depth behind the controller.
- Disallow privileged, hostNetwork, hostPath, and privilege escalation.
- Restrict the forge-shared IngressGroup to Ingresses rendered by the Forge controller (§11).
- Reject an Ingress whose host is already claimed by another Ingress in the forge-shared group, so one service can never take over another's traffic (§11).

Run Kyverno in Audit mode first and read the policy reports before switching to Enforce. Turning on enforcement across a working cluster and watching every deployment fail is a memorable but avoidable afternoon.

## 17. Observability

```text
Application  --> /metrics (Prometheus client library)
     |
     v
ServiceMonitor  (rendered by the controller for every service)
     |
     v
Prometheus  <-- ALB target-group metrics via CloudWatch exporter
     |              (RequestCount, HTTPCode_Target_5XX, TargetResponseTime)
     +--> recording rules: SLI, error budget, burn rate
     +--> alerting rules: multi-window multi-burn-rate
     |
     v
Grafana  --> per-service dashboard, generated from a template
```

Every Forge service receives, with no configuration beyond observability.metricsPort:

- Request rate, error rate, and latency percentiles at the load balancer.
- CPU and memory against requests and limits.
- Replica availability and pod restart counts.
- HPA scaling activity and current position between min and max.
- Availability, error budget remaining, and burn rate.
- A Grafana dashboard, provisioned from a template with the service name substituted.

> **Scope decision.** OpenTelemetry tracing is deferred. Metrics and SLOs fit the budget; a trace backend does not, and half-instrumented tracing is worse than none. §24 records the omission and what adding tracing would take.

**As configured.** Prometheus selects ServiceMonitors from every namespace and release, so a service needs no special labels to be scraped. The monitoring CRDs install from a separate Application and the chart's admission webhooks are disabled (see §14). Data is kept for 24 hours on an emptyDir since it would not survive teardown anyway, and Alertmanager is off until alerting arrives with the SLOs. Grafana is reached by port-forward, runs with a 512Mi memory limit, and its admin password comes from Secrets Manager through ESO.

**The request metrics contract.** Every service exposes http_requests_total and http_request_duration_seconds, labelled by method, matched route pattern and status code, on its metrics port. Labelling by route pattern rather than raw path keeps the number of series bounded when scanners hit random URLs through the ALB, and health-check paths are excluded so frequent probes do not dilute the success rate. The sample service's ERROR_RATE setting injects 500s, and an injected 10% measured as 11% in Prometheus, which validates the path from request to SLI end to end.

## 18. SLOs and Error Budgets

Revision 1 proposed Kubernetes readiness as the availability SLI and explicitly rejected request success rate. That is the wrong way round, for three reasons:

1. A pod can be Ready while returning 500 to every request. Readiness measures whether Kubernetes should route traffic to a pod, not whether the pod is serving that traffic correctly. It is a liveness signal wearing an availability costume.
2. Readiness is sampled at the Prometheus scrape interval, typically 30 seconds. Against a 43.2-minute monthly budget, that resolution is coarse and short outages are invisible entirely.
3. With ten replicas, "readiness = 1" is ambiguous. If it means any replica is ready, the SLI reports full availability while 90% of capacity is down.

### 18.1 The SLI

Availability is the proportion of requests served successfully, measured at the load balancer — the closest point to the user that the platform controls.

```text
# recording rule
forge:sli_availability:ratio_rate5m =
    sum(rate(aws_applicationelb_request_count_sum{service="$svc"}[5m]))
  - sum(rate(aws_applicationelb_httpcode_target_5xx_sum{service="$svc"}[5m]))
  /
    sum(rate(aws_applicationelb_request_count_sum{service="$svc"}[5m]))

# 30-day rolling error budget, 99.9% target
forge:error_budget_remaining:ratio =
  1 - (
    (1 - avg_over_time(forge:sli_availability:ratio_rate5m[30d]))
    / 0.001
  )
```

4xx responses are excluded from the error count. A client sending a malformed request is not a platform failure, and counting it as one makes the SLO measure the wrong system.

### 18.2 The error budget

```text
30 days x 24 hours x 60 minutes x 0.1%  =  43.2 minutes per 30-day window
```

Forge exposes consumed budget, remaining budget, current availability, and burn rate through both the Grafana dashboard and forge slo. Readiness and replica availability remain on the dashboard as secondary diagnostic signals — useful for explaining why the SLI moved, not for defining it.

### 18.3 Burn-rate alerting

Alert on multi-window multi-burn-rate conditions rather than on threshold breaches, which is the standard approach and avoids both alert fatigue and slow detection:

| Severity | Burn rate | Windows | Budget consumed before firing | Meaning |
|---|---|---|---|---|
| Page | 14.4x | 5m and 1h | 2% | Budget exhausted in ~2 days. Wake someone. |
| Page | 6x | 30m and 6h | 5% | Budget exhausted in ~5 days. |
| Ticket | 3x | 2h and 1d | 10% | Budget exhausted in ~10 days. |
| Ticket | 1x | 6h and 3d | 10% | Trending toward exhaustion at month end. |

The short window is the confirmation window: both must be burning before the alert fires, which suppresses single-spike noise while still catching a genuine fast burn within minutes.

## 19. Environment Model

Revision 1 did not address environments. Under a \$50 ceiling the answer is a single cluster with namespace separation, and the important part is being explicit that this is a budget decision rather than a recommendation.

| Aspect | This build | What production would do |
|---|---|---|
| Isolation | Namespace per service per environment in one cluster. | Separate clusters, often separate AWS accounts. |
| Promotion | PR copying an image tag from the dev overlay to the prod overlay. | Same mechanism. This part genuinely scales. |
| Config | Kustomize overlays per environment over a common ForgeService CR base. | Same. |
| Blast radius | Shared control plane, shared node pool, shared ALB. | Isolated per environment. |
| Access | One RBAC model, namespace-scoped roles. | Separate credentials per environment. |

The GitOps repository layout that supports this:

```text
gitops/
├── base/
│   └── payments/
│       └── forgeservice.yaml      # ForgeService CR, no image tag
├── overlays/
│   ├── dev/
│   │   └── payments/
│   │       └── kustomization.yaml # image tag, minReplicas 1, no SLO
│   └── prod/
│       └── payments/
│           └── kustomization.yaml # image tag, minReplicas 2, SLO 99.9
└── applicationset.yaml            # git directory generator over overlays/*/*
```

CI writes only to the dev overlay. Promotion to prod is always a human-merged pull request, so an automated pipeline can never push straight to production.

## 20. Reliability and Failure Testing

The point is not that Forge can deploy a workload. It is that workloads Forge creates recover predictably, and that the platform can measure what the failure cost.

| Experiment | Expected behavior | What it proves |
|---|---|---|
| Delete pods in a loop | PDB limits voluntary disruption to one pod at a time; ALB drains connections; SLI dips measurably but within budget. | PDB and readiness gates are correctly configured. |
| Drain a node | Pods reschedule; Karpenter provisions replacement capacity; no 5xx at the ALB. | Capacity management works without human intervention. |
| Force spot interruption | Karpenter handles the 2-minute notice, cordons and drains ahead of reclaim. | Spot-first is a safe default, not a cost gamble. |
| Break the readiness endpoint | Pod removed from the target group; traffic shifts to healthy replicas; SLI stays flat. | Health checking is wired end to end. |
| Return 500 from the app | SLI drops, error budget burns, burn-rate alert fires within the expected window. | The SLO measures reality — the single most important test here. |
| Scale load past maxReplicas | HPA saturates; latency degrades; NodePool ceiling holds; no runaway spend. | Cost guardrails hold under pressure. |
| terraform destroy then terraform apply | Full platform returns; Argo re-reconciles every service; DNS and TLS resolve. | The infrastructure code is genuinely complete. |

The last row matters most, and it is exercised every week by the cost discipline in §4. Failure experiments are recorded with timestamps and the matching Grafana panels, so each result is backed by evidence rather than assertion.

## 21. Repository Structure

```text
forge/
├── cmd/
│   ├── forge/                  # CLI entrypoint
│   └── forge-controller/       # controller entrypoint
├── pkg/
│   └── contract/v1alpha1/      # THE contract. Single source of truth.
├── internal/
│   ├── controller/             # reconciliation logic and renderers
│   ├── gitops/  status/  slo/
├── config/                     # generated CRDs, RBAC
├── infra/terraform/
│   ├── main.tf  terraform.tf  providers.tf  backend.tf   # main root
│   ├── makefile                # make up / down / plan / validate
│   ├── scripts/                # k8s-teardown.sh, check-leftovers.sh
│   ├── persistent/             # what survives teardown
│   └── modules/                # vpc eks iam karpenter argocd
│                               #   + ecr github-oidc acm (persistent)
├── charts/
│   ├── platform/               # app of apps: one Application per component
│   ├── karpenter-config/       # NodePool + EC2NodeClass
│   ├── external-secrets/       # ClusterSecretStore
│   └── payments/               # Kustomize: one service, by hand (Phase 3)
├── examples/payments-api/      # reference service, its own Go module
│   ├── cmd/payments-api/       # main: two servers, graceful shutdown
│   ├── internal/api/           # health, readiness, root handlers
│   ├── internal/metrics/       # request metrics middleware
│   └── Dockerfile
├── .github/workflows/          # build-and-test (PRs), deploy (main)
├── policies/kyverno/
├── observability/
├── docs/                       # architecture, service checklist, contract decisions
└── README.md
```

Three structural notes. First, there is no charts/forge-service — application rendering belongs to the controller, and keeping a parallel Helm implementation is the duplication §2 removed; charts/payments is the hand-written specification the controller will reproduce, not a template. Second, the sample service is its own Go module, so its dependencies stay out of the platform's. Third, there is no separate ADR folder. The contested decisions are argued in place in this document, and the contract decisions are logged with their reasoning in docs/service-contract-decisions.md. The contested ones are: Terraform modules over Pulumi, controller over Helm, success rate over readiness, no REST API, single NAT gateway, ESO over CSI driver, one workload account under an Organization, and sync waves ordered by capacity.

## 22. Cost Model

Rates are for us-east-1, which has the same list prices as us-east-2 for every line below, checked September 2026. The model assumes teardown between working sessions.

### 22.1 Hourly burn while the cluster is up

| Line item | Rate | Per hour |
|---|---|---|
| EKS control plane (standard support) | \$0.10 / hr | \$0.1000 |
| NAT gateway x1 | \$0.045 / hr | \$0.0450 |
| Shared ALB x1 | \$0.0225 / hr | \$0.0225 |
| System node group, 2x t3a.small on-demand | \$0.0188 / hr each | \$0.0376 |
| Karpenter spot capacity, ~2x t3a.medium | ~\$0.0125 / hr each | \$0.0250 |
| EBS gp3, ~60 GB attached | \$0.08 / GB-mo | \$0.0066 |
| TOTAL |   | \$0.2367 / hr |

### 22.2 Residue that survives teardown

| Line item | Per month |
|---|---|
| Route 53 hosted zone | \$0.50 |
| ECR storage, ~4 GB with lifecycle policy | \$0.40 |
| Secrets Manager, 3 secrets | \$1.20 |
| S3 (Terraform state) and retained CloudWatch logs | \$1.00 |
| NAT data processing and egress, ~20 GB | \$2.00 |
| TOTAL | \$5.10 |

### 22.3 Monthly total by uptime discipline

| Cluster uptime | Compute | Residue | Monthly total | Against \$50 |
|---|---|---|---|---|
| 40 hr/mo (~9 hr/week) | \$9.47 | \$5.10 | \$14.57 | Comfortable |
| 60 hr/mo (~14 hr/week) | \$14.20 | \$5.10 | \$19.30 | Comfortable — the planning assumption |
| 100 hr/mo (~23 hr/week) | \$23.67 | \$5.10 | \$28.77 | Fine |
| 150 hr/mo (~35 hr/week) | \$35.51 | \$5.10 | \$40.61 | Approaching the ceiling |
| 190 hr/mo (~44 hr/week) | \$44.97 | \$5.10 | \$50.07 | At the ceiling |
| 730 hr/mo (never torn down) | \$172.79 | \$5.10 | \$177.89 | Over by 3.6x |

> **The finding.** Under teardown discipline the budget is not tight — there is roughly 3x headroom over the realistic working pattern. Without teardown it is unreachable by a factor of 3.6, and the EKS control plane alone consumes 146% of it. This is why teardown automation is Phase 1 work with an exit criterion, not a cleanup task deferred to the end.

Practical controls: make up and make down in infra/terraform; a pre-destroy script that pauses Argo CD and deletes Ingresses, LoadBalancer Services, PersistentVolumeClaims, PodDisruptionBudgets and then Karpenter NodePools, so their controllers remove the AWS resources behind them before terraform destroy and no budget can hold a node drain open; a post-destroy check that fails if EBS volumes, Karpenter instances, Load Balancer Controller security groups or load balancers were left behind, since volumes do not block the destroy and keep billing, and a leftover instance holds the cluster security group and blocks VPC deletion (the check also runs on its own, which is the first step when a destroy hangs); an ECR lifecycle policy capping untagged images; CloudWatch log retention set to 7 days; Karpenter NodePool cpu and memory limits; and an AWS Budgets alert at \$35 wired to email. The budget alert is five minutes of work and is the thing that catches the mistake this model does not anticipate. Teardown also waits for ExternalDNS to remove its records before terraform destroy.

## 23. Definition of Done

Forge is complete when this sequence runs end to end with no manual Kubernetes or AWS interaction:

```text
$ make up                          # platform from nothing, < 30 minutes
$ forge create payments            # scaffold + PR
$ gh pr merge                      # register the service
$ git push                         # application code change
      |
      v
  tests -> SAST -> build -> Trivy -> SBOM -> cosign -> ECR
      |
      v
  GitOps dev overlay updated automatically
      |
      v
  Argo CD syncs -> ForgeService CR -> Forge controller
      |
      v
  Namespace, Deployment, Service, ServiceAccount, Ingress,
  HPA, PDB, NetworkPolicy, ServiceMonitor
      |
      v
  Shared ALB + Route 53 record + ACM certificate
      |
      v
  https://payments-dev.example.com  serving traffic
      |
$ forge promote payments --to prod
      |
      v
  https://payments.example.com  serving traffic
      |
$ forge slo payments
  availability 99.97%  budget consumed 26.6%  remaining 31.7 min  burn 0.4x
      |
$ make chaos                       # kill pods, drain nodes, inject 500s
      |
      v
  Grafana shows the dip, the budget burn, and the alert firing
      |
$ make down                        # back to $5.10/month
```

Every step above is observable and can be verified independently in under an hour; that is the acceptance test for the platform.

## 24. Open Questions and Deferred Decisions

Deferred and open decisions are recorded with their rationale so they can be revisited deliberately rather than rediscovered.

| Item | Status | Resolution or rationale |
|---|---|---|
| Terraform bootstrap chicken-and-egg | Resolved | The persistent root (OIDC provider, CI role, ECR template) is applied locally with Identity Center admin credentials. CI only builds and pushes images and never runs Terraform, so there is no loop. If CI later runs terraform plan, it gets a separate read-only role. |
| Terraform state backend | Decided | S3 backend with native state locking (use_lockfile); no DynamoDB table, since DynamoDB locking is deprecated. The state bucket is created once outside the main configuration and survives teardown. |
| OpenTelemetry tracing | Deferred | Metrics and SLOs fit the budget; a trace backend does not. The extension point is recorded here. |
| Contract versioning and conversion | Deferred | Shipping v1alpha1 with an explicit no-compatibility-guarantee note. Conversion webhooks only after the schema stabilizes. |
| Cluster upgrade procedure | Open | Exercised implicitly by teardown and rebuild. The STANDARD upgrade policy upgrades the control plane at the end of standard support; node AMIs are pinned in the EC2NodeClass and bumped deliberately. Write it up in Phase 9 as a runbook. |
| Secrets rotation | Deferred | ESO refresh interval handles propagation; rotation policy in Secrets Manager is out of scope. |
| Multi-cluster and DR | Out of scope | Single cluster, single region. Recovery time objective: a full rebuild takes about 30 minutes plus DNS propagation. |
| Progressive delivery | Out of scope | Argo Rollouts is the obvious next step and the controller is structured so it could render a Rollout instead of a Deployment. |
| Multi-architecture images | Deferred | The NodePool is restricted to amd64 until CI builds multi-architecture images. Graviton instances would then lower compute cost. |
| Second runtime: ECS on Fargate | Future | Keep EKS as the control plane; the controller renders AWS Controllers for Kubernetes (ACK) resources for services placed on ECS, so it still only writes Kubernetes objects. Open design question: the shared ALB, which the Load Balancer Controller owns. Estimate 80 to 120 hours for parity, 25 to 35 for a proof of concept behind its own ALB. |
| Golden paths for other languages (.NET first) | Future | The contract is language-neutral and must stay so. A second language needs a forge create template, a CI workflow variant (tests and dependency scanning; build, scan, sign and push stay shared) and a minimal base image. Estimate 10 to 15 hours. |
| AWS access for services | Decided | Not part of the contract. A service that calls AWS gets an IAM role and Pod Identity association from Terraform, bound to its ServiceAccount (<svc>-sa in its namespace). The controller stays limited to Kubernetes objects (§7), and every AWS permission goes through a reviewed Terraform change. ACK remains an option if self-service AWS access is ever wanted. Built when the first service needs AWS access; the design is in §9. |
| GitOps repository credential | Open | The workflow token cannot push to another repository. Phase 5 needs a deploy key on the GitOps repository or a GitHub App. |
| terraform plan in CI | Deferred | Needs a separate read-only role with state access. Only the persistent root's plan is meaningful; the main root is usually torn down. |
| Account-specific values | Decided | Installation values (account ID, region, cluster name, domain, registry, subnet CIDRs) come only from Terraform, passed as values on the root Application it already creates, so the §7 boundary is unchanged. charts/platform becomes a Helm chart that templates the platform Applications and, from Phase 4, the controller's ConfigMap, which the CLI also reads. Per-service values stay in the contract. Scheduled just before Phase 4. |
| Monitoring CRDs | Resolved | The CRDs install from a separate prometheus-operator-crds Application at wave 0, version-matched to kube-prometheus-stack and protected from pruning (§14). |

**Future extensions.** The ECS and other-language rows are candidates once the initial delivery is complete, because each proves the contract from a different side: the same Service runs on a second runtime, or the same pipeline builds a second language. Both are cheaper to keep possible than to retrofit, which is why the contract stays free of runtime and language details and the controller already mounts a writable /tmp.

## 25. Design Principles in Brief

Forge is a platform with a product decision at its center: what developers are allowed to express. Three principles follow from that and shape the rest of the design; each is covered in depth in its own section.

- There is no deploy command (§14). Every change is a Git commit, so the audit trail, the review gate and the rollback mechanism are the same mechanism. An imperative deploy API would be the anti-pattern the GitOps layer exists to prevent.
- Availability is measured at the load balancer, not by readiness (§18). A pod can be Ready and serving 500s; readiness tells Kubernetes where to route traffic, not whether users received an answer.
- There is no REST API (§2). The Kubernetes API server already provides authentication, RBAC, watch semantics, admission control and schema versioning. A second control plane in front of it would be a worse one.

## 26. Build Strategy

The control plane is not built first. The golden path is proven by hand, then the controller automates what was proven. The detailed schedule, estimates, dependencies, and cut lines are in the accompanying project plan; the sequencing logic is here.

1. AWS foundation with Terraform modules, including teardown automation. Nothing else starts until make up and make down round-trip cleanly.
2. Platform components through Argo CD: Karpenter, LB Controller, ExternalDNS, Kyverno in audit mode, kube-prometheus-stack.
3. Deploy one sample application with hand-written YAML through the intended GitOps path. Prove ALB, DNS, TLS, and Argo reconciliation work before abstracting over them.
4. Define the contract as Go types. Generate the CRD. Build the controller. This is the centerpiece and it gets the largest single block of time.
5. CI/CD with the full supply chain: scan, SBOM, keyless signing, GitOps write-back.
6. Kyverno to Enforce, with image verification bound to the signing identity from step 5.
7. Observability and the SLO implementation: recording rules, error budget, burn-rate alerts, generated dashboards.
8. The CLI, deliberately last and deliberately thin. By this point it is a convenience layer over things that already work.
9. Failure testing and operational runbooks.

> **Why the controller moved from last to fourth.** In Revision 1 the controller was Phase 8 of 9 — the first thing to be cut when time ran short, and the component that defines the platform. Here it is built immediately after the golden path is proven, and the CLI absorbs the schedule risk instead. Without the CLI, Forge is still a Kubernetes-native platform; without the controller, it is a collection of hand-written manifests.

Each step is a legitimate stopping point that leaves a working platform. The platform is usable after step 4 and substantially complete after step 7. That property — delivering value at every stage rather than only at the end — is what makes a 146-hour plan workable inside 130 hours of capacity.
