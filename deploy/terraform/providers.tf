terraform {
    required_version = ">= 1.5.0"

    required_providers {
        # take to Kubernetes API 
        kubernetes = {
            source = "hashicorp/kubernetes"
            version = "~> 2.35"
        }
        null = {
            source = "hashicorp/null"
            version = "~> 3.2"
        }
    }
}

provider "kubernetes" {
    config_path = "~/.kube/config"
    config_context = "minikube"
}