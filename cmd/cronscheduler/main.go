package main

import (
	"context"
	"flag"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/coldog/tool-ecs/internal/kv"
	"log"
	"os"
	"os/signal"
)

var region string

func main() {
	flag.StringVar(&region, "region", "us-west-2", "Aws Region")
	flag.Parse()

	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		log.Fatalf("[FATA] main: could not connect to aws -- %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	db, err := kv.NewDynamoDB(sess)
	if err != nil {
		log.Fatalf("[FATA] main: could not connect to dynamo -- %v", err)
	}

	sched := &scheduler{
		ctx: ctx,
		kv:  db,
		ecs: NewECSClient(sess),
	}

	sigs := make(chan os.Signal)
	signal.Notify(sigs, os.Interrupt, os.Kill)

	go func() {
		sig := <-sigs
		log.Printf("[WARN] main: exiting due to %v", sig)
		cancel()
		os.Exit(0)
	}()

	sched.run()
}
