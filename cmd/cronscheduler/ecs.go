package main

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"log"
	"time"
)

type ECSClient interface {
	Open(ctx context.Context) error
	RunTask(cluster, taskDefinition string) error
}

func NewECSClient() ECSClient {
	return &ecsClient{}
}

type ecsClient struct {
	ecs *ecs.ECS
}

func (ecsClient *ecsClient) Open(ctx context.Context) error {
	for {
		sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
		if err == nil {
			ecsClient.ecs = ecs.New(sess)
			log.Println("[INFO] main: connected to aws")
			return nil
		}

		log.Printf("[WARN] main: could not connect to ecs, trying again in 3 seconds -- %v", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func (ecsClient *ecsClient) RunTask(cluster, taskDefinition string) error {
	_, err := ecsClient.ecs.RunTask(&ecs.RunTaskInput{
		Cluster:        aws.String(cluster),
		TaskDefinition: aws.String(taskDefinition),
		StartedBy:      aws.String("CronScheduler"),
	})
	return err
}
