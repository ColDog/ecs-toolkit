provider "aws" {
  region = "us-west-2"
}

data "aws_ami" "main" {
  most_recent = true

  filter {
    name   = "name"
    values = ["amzn-ami-2016.09.g-amazon-ecs-optimized"]
  }
}

resource "aws_launch_configuration" "main" {
  name                  = "main-7"
  image_id              = "${data.aws_ami.main.id}"
  instance_type         = "t2.micro"
  user_data             = "#!/bin/bash\necho ECS_CLUSTER=sked >> /etc/ecs/ecs.config"
  key_name              = "sked"
  iam_instance_profile  = "ecsInstanceRole"
  security_groups       = ["sg-f7785c91"]

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_autoscaling_group" "main" {
  availability_zones        = ["us-west-2a", "us-west-2b", "us-west-2c"]
  name                      = "sked"
  max_size                  = 2
  min_size                  = 1
  desired_capacity          = 2
  force_delete              = true
  launch_configuration      = "${aws_launch_configuration.main.name}"

  tag {
    key                 = "cluster"
    value               = "main"
    propagate_at_launch = true
  }
}

resource "aws_ecs_cluster" "main" {
  name = "sked"
}
