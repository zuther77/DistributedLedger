variable "namespace" {
    type = string
    default = "ledger"
    description = "K8s namespace for trading stack"
}

variable "k8s_dir" {
    type = string
    default = "../k8s"
    description = "Folder of raw YAML"
}