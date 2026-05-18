variable "settings" {}

variable "env_input" {
  type = string
}

module "mod" {
  source = "../modules/one"
}

resource "random_integer" "this" {
  min = 1
  max = 50000
}

resource "random_integer" "include" {
  min = 1
  max = 50000
}

resource "random_integer" "exclude" {
  min = 1
  max = 50000
}

output "var1" {
  value = var.settings.name
}

output "var2" {
  value = var.settings.dns.domain
}
