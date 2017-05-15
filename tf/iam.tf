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
