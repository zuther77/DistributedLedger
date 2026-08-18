# 1) Terraform directly manages the namespace object.
resource "kubernetes_namespace" "ledger" {
  metadata {
    name = var.namespace
  }
}

# 2) After namespace exists, apply all K8s YAML.
resource "null_resource" "apply_manifests" {
  depends_on = [kubernetes_namespace.ledger]

  triggers = {
    # Bump manually when YAML changes, OR add filesha256 per file.
    force_version = "1"
  }

  provisioner "local-exec" {
    command = "kubectl apply -f ${var.k8s_dir}/"
  }
}
