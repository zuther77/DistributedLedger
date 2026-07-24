output "port_forward_hint" {
    value = "kubectl -n ${var.namespace} port-forward svc/order-api 8080:8080"
}

output "get_pods_hint" {
  value = "kubectl -n ${var.namespace} get pods"
}