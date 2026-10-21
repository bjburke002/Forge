resource "helm_release" "argo" {
  name             = "argocd"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-cd"
  version          = "10.10.5"
  namespace        = "argocd"
  create_namespace = true

  values = [
    file("${path.module}/values/argo.values.yaml")
  ]
}

resource "helm_release" "root" {
  name             = "argocd-apps"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argocd-apps"
  version          = "2.0.7"
  namespace        = "argocd"
  create_namespace = true

  values = [yamlencode({
    applications = {
      platform = {
        project = "default"
        source = {
          repoURL        = var.repo_url
          targetRevision = var.revision
          path           = var.path # charts/platform
        }
        destination = {
          server    = "https://kubernetes.default.svc"
          namespace = "argocd"
        }
        syncPolicy = {
          automated = { prune = true, selfHeal = true }
        }
        # no finalizers, so make down can't hang
      }
    }
  })]

  depends_on = [helm_release.argo]
}