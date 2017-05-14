package actions

import (
	"context"
	"flag"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
	"io"
	"time"
)

type Scale struct {
	flag    *flag.FlagSet
	Region  string
	Cluster string
	Service string
	Count   int64
	ecs     *ecs.ECS
}

func (cmd *Scale) ShortDescription() string { return "Scale a service" }
func (cmd *Scale) PrintUsage()              { cmd.flag.PrintDefaults() }

func (cmd *Scale) ParseArgs(args []string) {
	cmd.flag = flag.NewFlagSet("Apply", flag.ExitOnError)
	cmd.flag.StringVar(&cmd.Region, "region", "us-west-2", "AWS Region")
	cmd.flag.StringVar(&cmd.Cluster, "cluster", "default", "AWS ECS Cluster")
	cmd.flag.StringVar(&cmd.Service, "service", "", "Service to scale")
	cmd.flag.Int64Var(&cmd.Count, "count", 1, "Desired count")
	cmd.flag.Parse(args)
}

func (cmd *Scale) Run(w io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sess, err := getSession(cmd.Region)
	if err != nil {
		return errors.Wrap(err, "Could not open aws session")
	}
	cmd.ecs = ecs.New(sess)

	_, err = cmd.ecs.UpdateServiceWithContext(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(cmd.Cluster),
		Service:      aws.String(cmd.Service),
		DesiredCount: aws.Int64(cmd.Count),
	})
	return err
}
