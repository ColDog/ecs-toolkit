package actions

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/coldog/tool-ecs/internal/kv"
	"github.com/pkg/errors"
	"os"
)

type Spec struct {
	Type string          `json:"type"`
	ID   string          `json:"id"`
	Spec json.RawMessage `json:"spec"`
}

type Apply struct {
	flag   *flag.FlagSet
	file   string
	region string
	ecs    *ecs.ECS
}

func (cmd *Apply) ShortDescription() string { return "Apply a resource" }

func (cmd *Apply) ParseArgs(args []string) {
	cmd.flag = flag.NewFlagSet("Apply", flag.ExitOnError)
	cmd.flag.StringVar(&cmd.region, "region", "us-west-2", "AWS Region")
	cmd.flag.Parse(args)
	cmd.file = cmd.flag.Arg(0)
}

func (cmd *Apply) Run() error {
	f, err := os.OpenFile(cmd.file, os.O_RDONLY, 0600)
	if err != nil {
		return errors.Wrap(err, "Could not open file")
	}

	sess, err := getSession(cmd.region)
	if err != nil {
		return errors.Wrap(err, "Could not open aws session")
	}

	spec := &Spec{}
	err = json.NewDecoder(f).Decode(spec)
	if err != nil {
		return errors.Wrap(err, "Could not decode file")
	}

	ecsClient := ecs.New(sess)
	kvClient, err := kv.NewDynamoDB(sess)
	if err != nil {
		return errors.Wrap(err, "Could not open dynamodb session")
	}

	cmd.ecs = ecsClient

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	switch spec.Type {
	// Dynamo resources:
	case "CronJob":
		return kvClient.Put(ctx, spec.Type, spec.ID, spec.Spec)

	// ECS Resources:
	case "TaskDefinition":
		return cmd.handleTaskDefinition(ctx, spec)
	case "Service":
		return cmd.handleService(ctx, spec)
	default:
		return errors.Errorf("Could not recognize type %s", spec.Type)
	}
}

func (cmd *Apply) handleTaskDefinition(ctx context.Context, spec *Spec) error {
	input := &ecs.RegisterTaskDefinitionInput{}
	err := json.Unmarshal(spec.Spec, input)
	if err != nil {
		return err
	}
	_, err = cmd.ecs.RegisterTaskDefinitionWithContext(ctx, input)
	return err
}

func (cmd *Apply) handleService(ctx context.Context, spec *Spec) error {
	out, err := cmd.ecs.DescribeServices(&ecs.DescribeServicesInput{
		Services: []*string{aws.String(spec.ID)},
	})
	if err != nil {
		return err
	}
	if len(out.Services) == 0 {
		input := &ecs.CreateServiceInput{}
		err = json.Unmarshal(spec.Spec, input)
		if err != nil {
			return err
		}
		_, err = cmd.ecs.CreateServiceWithContext(ctx, input)

	} else {
		input := &ecs.UpdateServiceInput{}
		err = json.Unmarshal(spec.Spec, input)
		if err != nil {
			return err
		}
		_, err = cmd.ecs.UpdateServiceWithContext(ctx, input)
	}
	return err
}
