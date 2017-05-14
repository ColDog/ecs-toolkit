package actions

import (
	"context"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
	"io"
	"strings"
	"time"
)

type Deploy struct {
	flag       *flag.FlagSet
	Region     string
	Cluster    string
	Service    string
	Count      int64
	Containers []string
	ecs        *ecs.ECS
}

func (cmd *Deploy) ShortDescription() string { return "Scale a service" }
func (cmd *Deploy) PrintUsage()              { cmd.flag.PrintDefaults() }

func (cmd *Deploy) ParseArgs(args []string) {
	cmd.flag = flag.NewFlagSet("Apply", flag.ExitOnError)
	cmd.flag.StringVar(&cmd.Region, "region", "us-west-2", "AWS Region")
	cmd.flag.StringVar(&cmd.Cluster, "cluster", "default", "AWS ECS Cluster")
	cmd.flag.Int64Var(&cmd.Count, "count", 1, "Desired count")
	cmd.flag.StringVar(&cmd.Service, "service", "", "Service name")
	cmd.flag.Parse(args)
	cmd.Containers = cmd.flag.Args()
}

func (cmd *Deploy) Run(w io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sess, err := getSession(cmd.Region)
	if err != nil {
		return errors.Wrap(err, "Could not open aws session")
	}
	cmd.ecs = ecs.New(sess)

	svcs, err := cmd.ecs.DescribeServices(&ecs.DescribeServicesInput{
		Cluster:  aws.String(cmd.Cluster),
		Services: []*string{aws.String(cmd.Service)},
	})
	if err != nil {
		return errors.Wrap(err, "Could not get services")
	}
	if len(svcs.Services) == 0 {
		return errors.Errorf("Service %s not found", cmd.Service)
	}
	svc := svcs.Services[0]

	// Containers
	containerUpdates := map[string]string{}
	for _, container := range cmd.Containers {
		parts := strings.Split(container, "=")
		if len(parts) != 2 {
			return errors.Errorf("Containers must be described as <name>=<image:tag>, received: %s", container)
		}
		containerUpdates[parts[0]] = parts[1]
	}

	io.WriteString(w, fmt.Sprintf("Updating containers: %+v\n", containerUpdates))

	// Get the current task definition
	taskDefDesc, err := cmd.ecs.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
		TaskDefinition: svc.TaskDefinition,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to describe task definition")
	}

	taskDef := taskDefDesc.TaskDefinition

	// Replace container images
	for _, container := range taskDef.ContainerDefinitions {
		if image, ok := containerUpdates[*container.Name]; ok {
			container.Image = aws.String(image)
		}
	}

	// Register a new task definition
	taskDefUpdate, err := cmd.ecs.RegisterTaskDefinition(&ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions: taskDef.ContainerDefinitions,
		Family:               taskDef.Family,
		NetworkMode:          taskDef.NetworkMode,
		PlacementConstraints: taskDef.PlacementConstraints,
		TaskRoleArn:          taskDef.TaskRoleArn,
		Volumes:              taskDef.Volumes,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to update task definition")
	}

	taskDefId := *taskDefUpdate.TaskDefinition.TaskDefinitionArn

	io.WriteString(w, "Updating service with new task definition "+taskDefId)

	// Update the service
	_, err = cmd.ecs.UpdateServiceWithContext(ctx, &ecs.UpdateServiceInput{
		Cluster:        aws.String(cmd.Cluster),
		Service:        aws.String(cmd.Service),
		DesiredCount:   aws.Int64(cmd.Count),
		TaskDefinition: aws.String(taskDefId),
	})
	return err
}
