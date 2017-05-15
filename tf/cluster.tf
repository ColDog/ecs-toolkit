variable "cluster_name" {
  type = "string"
  default = "default"
}

variable "instance_size" {
  type = "string"
  default = "t2.micro"
}

variable "autoscaling_group_version" {
  type = "string"
  default = "1"
}

variable "autoscaling_group_name" {
  type = "string"
  default = "default_group"
}

variable "autoscaling_group_max" {
  type = "string"
  default = 2
}

variable "autoscaling_group_min" {
  type = "string"
  default = 1
}

variable "autoscaling_group_desired" {
  type = "string"
  default = 2
}

variable "aws_region" {
  type = "string"
  default = "us-west-2"
}

variable "state_bucket" {
  type = "string"
  default = "ecs-state"
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

resource "aws_iam_role" "ecs_instance" {
  name               = "ecs_role"
  assume_role_policy = "${file("tf/policies/ecs_role.json")}"
}

resource "aws_iam_role_policy" "ecs_instance" {
  name   = "ecs_instance_role_policy"
  policy = "${file("tf/policies/ecs_instance_policy.json")}"
  role   = "${aws_iam_role.ecs_instance.id}"
}

resource "aws_iam_instance_profile" "ecs_instance" {
  name = "ecs_instance_profile"
  path = "/"
  role = "${aws_iam_role.ecs_instance.name}"
}

data "aws_ami" "ecs_optimized" {
  most_recent = true

  filter {
    name   = "name"
    values = ["*amazon-ecs-optimized"]
  }
}

resource "aws_security_group" "ecs_instance" {
  name        = "ecs_instance"
  description = "ECS instance security group"

  ingress {
    from_port   = 0
    to_port     = 65535
    protocol    = "tcp"
    self        = true
  }

  egress {
    from_port       = 0
    to_port         = 0
    protocol        = "-1"
    cidr_blocks     = ["0.0.0.0/0"]
  }
}

resource "aws_launch_configuration" "ecs" {
  name                  = "${var.autoscaling_group_name}.${var.autoscaling_group_version}"
  image_id              = "${data.aws_ami.ecs_optimized.id}"
  instance_type         = "${var.instance_size}"
  user_data             = "#!/bin/bash\necho ECS_CLUSTER=${var.cluster_name} >> /etc/ecs/ecs.config"
  key_name              = "${var.cluster_name}-key"
  iam_instance_profile  = "${aws_iam_instance_profile.ecs_instance.id}"
  security_groups       = ["${aws_security_group.ecs_instance.id}"]

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_autoscaling_group" "ecs" {
  availability_zones        = ["us-west-2a", "us-west-2b", "us-west-2c"]
  name                      = "${var.autoscaling_group_name}"
  max_size                  = "${var.autoscaling_group_max}"
  min_size                  = "${var.autoscaling_group_min}"
  desired_capacity          = "${var.autoscaling_group_desired}"
  launch_configuration      = "${aws_launch_configuration.ecs.name}"
  force_delete              = true

  tag {
    key                 = "cluster"
    value               = "${var.cluster_name}"
    propagate_at_launch = true
  }
}

resource "aws_ecs_cluster" "cluster" {
  name = "${var.cluster_name}"
}
