variable "cluster_name" {
  type    = "string"
  default = "default"
}

variable "instance_size" {
  type    = "string"
  default = "t2.micro"
}

variable "autoscaling_group_version" {
  type    = "string"
  default = "1"
}

variable "autoscaling_group_name" {
  type    = "string"
  default = "default_group"
}

variable "autoscaling_group_max" {
  type    = "string"
  default = 2
}

variable "autoscaling_group_min" {
  type    = "string"
  default = 1
}

variable "autoscaling_group_desired" {
  type    = "string"
  default = 2
}

variable "aws_region" {
  type    = "string"
  default = "us-west-2"
}

variable "state_bucket" {
  type    = "string"
  default = "ecs-state"
}

variable "vpc_name" {
  type    = "string"
  default = "ecs"
}

variable "cidr_blocks" {
  type = "list"
  // Must have enough blocks for the amount of availability zones in the
  // region.
  default = [
    "10.0.0.0/24",
    "10.0.1.0/24",
    "10.0.2.0/24",
  ]
}

// Setup
provider "aws" {
  region = "${var.aws_region}"
}

terraform {
  backend "s3" {
    bucket = "ecs-state"
    key    = "terraform"
    region = "us-west-2"
  }
}

resource "aws_ecs_cluster" "main" {
  name = "${var.cluster_name}"
}
