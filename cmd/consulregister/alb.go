package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
)

type ALB interface {
	Register()
	DeRegister()
}

type alb struct {
	lbId   string
	vpcId  string
	listId string
	suffix string
	client *elbv2.ELBV2
}

func (alb *alb) Register(service string, port int, checkPath string) error {
	var targetGroupId string
	targetGroups, err := alb.client.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
		Names:           []*string{aws.String(service)},
		LoadBalancerArn: aws.String(alb.lbId),
	})
	if err != nil {
		return err
	}
	if len(targetGroups.TargetGroups) == 0 {
		targetGroupsRes, err := alb.client.CreateTargetGroup(&elbv2.CreateTargetGroupInput{
			HealthCheckPath: aws.String(checkPath),
			Name:            aws.String(service),
			Protocol:        aws.String("HTTP"),
			VpcId:           aws.String(alb.vpcId),
		})
		if err != nil {
			return err
		}
		targetGroupId = *targetGroupsRes.TargetGroups[0].TargetGroupArn
	} else {
		targetGroupId = *targetGroups.TargetGroups[0].TargetGroupArn
	}

	rules, err := alb.client.DescribeRules(&elbv2.DescribeRulesInput{
		ListenerArn: aws.String(alb.listId),
	})
	if err != nil {
		return err
	}

	hasRule := false
	for _, rule := range rules.Rules {
		for _, cond := range rule.Conditions {
			if *cond.Field == "host-header" && *cond.Values[0] == service+alb.suffix {
				hasRule = true
				break
			}
		}
	}

	if !hasRule {
		_, err = alb.client.CreateRule(&elbv2.CreateRuleInput{
			Conditions: []*elbv2.RuleCondition{
				{Field: aws.String("host-header"), Values: []*string{
					aws.String(service + alb.suffix),
				}},
			},
			ListenerArn: aws.String(alb.listId),
			Actions: []*elbv2.Action{
				{Type: aws.String("forward"), TargetGroupArn: aws.String(targetGroupId)},
			},
		})
		if err != nil {
			return err
		}
	}

	//alb.client.RegisterTargets(&elb)
}
