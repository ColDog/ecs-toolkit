data "aws_ami" "ecs_optimized" {
  most_recent = true

  filter {
    name   = "name"
    values = ["amzn-ami-2016.09.g-amazon-ecs-optimized"]
  }
}

resource "aws_launch_configuration" "main" {
  name                 = "${var.autoscaling_group_name}.${uuid()}"
  image_id             = "${data.aws_ami.ecs_optimized.id}"
  instance_type        = "${var.instance_size}"
  user_data            = "#!/bin/bash\necho ECS_CLUSTER=${var.cluster_name} > /etc/ecs/ecs.config"
  key_name             = "${var.cluster_name}_key"
  iam_instance_profile = "${aws_iam_instance_profile.ecs_instance.id}"
  security_groups      = ["${aws_security_group.main_instance.id}"]

  lifecycle {
    create_before_destroy = true
    ignore_changes = ["name"]
  }
}

resource "aws_autoscaling_group" "main" {
  name                 = "${var.autoscaling_group_name}"
  max_size             = "${var.autoscaling_group_max}"
  min_size             = "${var.autoscaling_group_min}"
  desired_capacity     = "${var.autoscaling_group_desired}"
  launch_configuration = "${aws_launch_configuration.main.name}"
  force_delete         = true
  vpc_zone_identifier  = ["${aws_subnet.main.*.id}"]

  tag {
    key                 = "cluster"
    value               = "${var.cluster_name}"
    propagate_at_launch = true
  }
}
