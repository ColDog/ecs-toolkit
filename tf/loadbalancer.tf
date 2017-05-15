resource "aws_route53_zone" "main" {
  name   = "${var.cluster_name}.local"
  vpc_id = "${aws_vpc.main.id}"
}

resource "aws_route53_record" "internal_lb" {
  zone_id = "${aws_route53_zone.main.zone_id}"
  name    = "*.${var.cluster_name}.local"
  type    = "A"

  alias {
    name                    = "${aws_alb.internal.dns_name}"
    zone_id                 = "${aws_alb.internal.zone_id}"
    evaluate_target_health  = false
  }
}

resource "aws_alb" "internal" {
  name      = "internal"
  subnets   = ["${aws_subnet.main.*.id}"]
  internal  = true

  security_groups = ["${aws_security_group.main_internal.id}"]
}

resource "aws_alb" "external" {
  name      = "external"
  subnets   = ["${aws_subnet.main.*.id}"]
  internal  = false

  security_groups = ["${aws_security_group.main_external.id}"]
}
