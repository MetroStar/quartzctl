variable "config_json" {
    type = string
}

variable "name" {
    type = string
}

output "name" {
    value = var.name
}
