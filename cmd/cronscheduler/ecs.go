package main

import (
	"context"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

type ECSClient interface {
	RunTask(ctx context.Context, input *ecs.RunTaskInput) error
}

func NewECSClient(sess *session.Session) ECSClient {
	return &ecsClient{
		ecs: ecs.New(sess),
	}
}

type ecsClient struct {
	ecs *ecs.ECS
}

func (ecsClient *ecsClient) RunTask(ctx context.Context, input *ecs.RunTaskInput) error {
	_, err := ecsClient.ecs.RunTask(input)
	return err
}
