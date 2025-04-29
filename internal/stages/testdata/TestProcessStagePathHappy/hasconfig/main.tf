variable "root_name" {
    type = string
}

variable "second_name" {
    type = string
}

variable "third_name" {
    type = string
}

output "names" {
    value = [
        var.root_name,
        var.second_name,
        var.third_name,
    ]
}
