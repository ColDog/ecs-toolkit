package main

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/pkg/errors"
	"strconv"
)

type ALB interface {
	Open() error
	Register(ctx context.Context, service string, port int, checkPath string) error
	DeRegister(ctx context.Context, service string, port int) error
}

func NewALB(sess *session.Session, name, suffix string) ALB {
	return &alb{
		LBName:     name,
		Suffix:     suffix,
		client:     elbv2.New(sess),
		metaClient: ec2metadata.New(sess),
	}
}

type alb struct {
	LBName string
	Suffix string

	lbId       string
	vpcId      string
	listId     string
	instanceId string

	client     *elbv2.ELBV2
	metaClient *ec2metadata.EC2Metadata
}

func (alb *alb) Open() error {
	if alb.instanceId == "" {
		info, err := alb.metaClient.GetInstanceIdentityDocument()
		if err != nil {
			return err
		}
		alb.instanceId = info.InstanceID
	}

	lbs, err := alb.client.DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
		Names: []*string{aws.String(alb.LBName)},
	})
	if err != nil {
		return err
	}
	if len(lbs.LoadBalancers) == 0 {
		return errors.Errorf("No load balancers found for name %s", alb.LBName)
	}
	lb := lbs.LoadBalancers[0]
	alb.vpcId = *lb.VpcId
	alb.lbId = *lb.LoadBalancerArn

	list, err := alb.client.DescribeListeners(&elbv2.DescribeListenersInput{
		LoadBalancerArn: lb.LoadBalancerArn,
	})
	if err != nil {
		return err
	}
	if len(list.Listeners) == 0 {
		return errors.Errorf("No listeners found for name %s", alb.LBName)
	}
	alb.listId = *list.Listeners[0].ListenerArn
	return nil
}

func (alb *alb) registerInTargetGroup(targetGroupId string, port int) error {
	_, err := alb.client.RegisterTargets(&elbv2.RegisterTargetsInput{
		TargetGroupArn: aws.String(targetGroupId),
		Targets: []*elbv2.TargetDescription{
			{Id: aws.String(alb.instanceId), Port: aws.Int64(int64(port))},
		},
	})
	return err
}

func (alb *alb) deregisterTargetGroup(targetGroupId string, port int) error {
	_, err := alb.client.DeregisterTargets(&elbv2.DeregisterTargetsInput{
		TargetGroupArn: aws.String(targetGroupId),
		Targets: []*elbv2.TargetDescription{
			{Id: aws.String(alb.instanceId), Port: aws.Int64(int64(port))},
		},
	})
	return err
}

func (alb *alb) getTargetGroup(service string, checkPath string) (string, error) {
	if checkPath == "" {
		checkPath = "/healthz"
	}

	var targetGroupId string
	targetGroups, _ := alb.client.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
		Names: []*string{aws.String(service)},
	})
	if len(targetGroups.TargetGroups) == 0 {
		targetGroupsRes, err := alb.client.CreateTargetGroup(&elbv2.CreateTargetGroupInput{
			HealthCheckPath: aws.String(checkPath),
			Name:            aws.String(service),
			Protocol:        aws.String("HTTP"),
			VpcId:           aws.String(alb.vpcId),
			Port:            aws.Int64(80),
		})
		if err != nil {
			return "", errors.Wrap(err, "Could not create target group")
		}
		targetGroupId = *targetGroupsRes.TargetGroups[0].TargetGroupArn
	} else {
		targetGroupId = *targetGroups.TargetGroups[0].TargetGroupArn
	}
	return targetGroupId, nil
}

func (alb *alb) ensureRuleExists(service, targetGroupId string) error {
	rules, err := alb.client.DescribeRules(&elbv2.DescribeRulesInput{
		ListenerArn: aws.String(alb.listId),
	})
	if err != nil {
		return err
	}

	var priority int64
	for _, rule := range rules.Rules {
		for _, cond := range rule.Conditions {
			if *cond.Field == "host-header" && *cond.Values[0] == service+alb.Suffix {
				return nil
			}
		}
		p, _ := strconv.ParseInt(*rule.Priority, 10, 0)
		if p > priority {
			priority = p
		}
	}

	_, err = alb.client.CreateRule(&elbv2.CreateRuleInput{
		Conditions: []*elbv2.RuleCondition{
			{Field: aws.String("host-header"), Values: []*string{
				aws.String(service + alb.Suffix),
			}},
		},
		Priority:    aws.Int64(priority + 1),
		ListenerArn: aws.String(alb.listId),
		Actions: []*elbv2.Action{
			{Type: aws.String("forward"), TargetGroupArn: aws.String(targetGroupId)},
		},
	})
	if err != nil {
		return errors.Wrap(err, "Could not get rule")
	}
	return err
}

func (alb *alb) Register(ctx context.Context, service string, port int, checkPath string) error {
	targetGroupId, err := alb.getTargetGroup(service, checkPath)
	if err != nil {
		return err
	}

	err = alb.ensureRuleExists(service, targetGroupId)
	if err != nil {
		return err
	}

	err = alb.registerInTargetGroup(targetGroupId, port)
	if err != nil {
		return err
	}

	return nil
}

func (alb *alb) DeRegister(ctx context.Context, service string, port int) error {
	targetGroupId, err := alb.getTargetGroup(service, "") // won't use the checkpath
	if err != nil {
		return err
	}

	err = alb.deregisterTargetGroup(targetGroupId, port)
	if err != nil {
		return err
	}

	return nil
}
