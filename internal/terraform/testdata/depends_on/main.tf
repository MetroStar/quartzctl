variable "value_input" {
  type = string
}

variable "env_input" {
  type = string
}

variable "config_input" {
  type = string
}

variable "secret_input" {
  type = string
}

variable "stage_input" {
  type = string
}

variable "config_not_found" {
  type = string
  default = "default"
}

variable "secret_not_found" {
  type = string
  default = "default"
}

variable "stage_not_founc" {
  type = string
  default = "default"
}