package actions

import (
	"context"
	"flag"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/coldog/tool-ecs/internal/kv"
	"github.com/pkg/errors"
	"io"
	"strings"
	"time"
)

type Remove struct {
	flag   *flag.FlagSet
	Type   string
	ID     string
	Region string
	ecs    *ecs.ECS
}

func (cmd *Remove) ShortDescription() string { return "Remove a resource" }
func (cmd *Remove) PrintUsage()              { cmd.flag.PrintDefaults() }

func (cmd *Remove) ParseArgs(args []string) {
	cmd.flag = flag.NewFlagSet("Apply", flag.ExitOnError)
	cmd.flag.StringVar(&cmd.Region, "region", "us-west-2", "AWS Region")
	cmd.flag.StringVar(&cmd.Type, "type", "", "Type to remove")
	cmd.flag.StringVar(&cmd.ID, "id", "", "ID to remove")
	cmd.flag.Parse(args)
}

func (cmd *Remove) Run(w io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sess, err := getSession(cmd.Region)
	if err != nil {
		return errors.Wrap(err, "Could not open aws session")
	}

	ecsClient := ecs.New(sess)
	kvClient, err := kv.NewDynamoDB(sess)
	if err != nil {
		return errors.Wrap(err, "Could not open dynamodb session")
	}

	cmd.ecs = ecsClient

	switch cmd.Type {
	case "CronJob":
		return kvClient.Del(ctx, cmd.Type, cmd.ID)
	case "TaskDefinition":
		return cmd.handleTaskDefinition(ctx, cmd.ID)
	case "Service":
		return cmd.handleService(ctx, cmd.ID)
	default:
		return errors.Errorf("Could not recognize type %s", cmd.Type)
	}
}

func (cmd *Remove) handleTaskDefinition(ctx context.Context, id string) error {
	_, err := cmd.ecs.DeregisterTaskDefinitionWithContext(ctx, &ecs.DeregisterTaskDefinitionInput{
		TaskDefinition: aws.String(id),
	})
	return err
}

func (cmd *Remove) handleService(ctx context.Context, id string) error {
	parts := strings.Split(id, "/")
	_, err := cmd.ecs.DeleteServiceWithContext(ctx, &ecs.DeleteServiceInput{
		Service: aws.String(parts[0]),
		Cluster: aws.String(parts[1]),
	})
	return err
}
