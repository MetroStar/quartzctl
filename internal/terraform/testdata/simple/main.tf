variable "settings" {}

variable "value_input" {
  type = string
}

output "var1" {
  value = var.settings.name
}

output "var2" {
  value = var.settings.dns.domain
}
