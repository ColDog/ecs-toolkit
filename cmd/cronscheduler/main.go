package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/gorhill/cronexpr"
	consul "github.com/hashicorp/consul/api"
	"log"
	"os"
	"os/signal"
	"time"
)

type ECSClient interface {
	RunTask(input *ecs.RunTaskInput) (*ecs.RunTaskOutput, error)
}

type ConsulKVClient interface {
	List(prefix string, opts *consul.QueryOptions) (consul.KVPairs, *consul.QueryMeta, error)
}

var KeyPrefix = "cronjobs/"

// A cron job represents a runnable cron job
type CronJob struct {
	// The last run executed by this job, used to find the next run.
	LastRun time.Time

	// Task definition ID to run.
	TaskDefinitionID string
	Cluster          string

	// A Cron string.
	Schedule string
}

type scheduler struct {
	ctx context.Context
	kv  ConsulKVClient
	ecs ECSClient
}

func (scheduler *scheduler) schedule(cluster, taskDefinitionId string) error {
	_, err := scheduler.ecs.RunTask(&ecs.RunTaskInput{
		Cluster:        aws.String(cluster),
		TaskDefinition: aws.String(taskDefinitionId),
		StartedBy:      aws.String("CronScheduler"),
	})
	return err
}

func (scheduler *scheduler) runJob(job *CronJob) error {
	expr, err := cronexpr.Parse(job.Schedule)
	if err != nil {
		return err
	}

	if job.LastRun.IsZero() {
		job.LastRun = time.Now()
	}
	next := expr.Next(job.LastRun)
	if next.IsZero() {
		return fmt.Errorf("ErrCronFormat: %s is invalid", job.Schedule)
	}

	if time.Now().After(next) {
		log.Printf("[INFO] scheduler: running task %s/%s", job.Cluster, job.TaskDefinitionID)
		return scheduler.schedule(job.Cluster, job.TaskDefinitionID)
	}
	return nil
}

func (scheduler *scheduler) evaluate() {
	keys, _, err := scheduler.kv.List(KeyPrefix, &consul.QueryOptions{RequireConsistent: true})
	if err != nil {
		log.Printf("[WARN] scheduler: failed to read keys -- %v", err)
		return
	}

	for _, key := range keys {
		job := &CronJob{}
		err := json.Unmarshal(key.Value, job)
		if err != nil {
			log.Printf("[WARN] scheduler: failed to read cron job at %s -- %v", key.Key, err)
			continue
		}

		err = scheduler.runJob(job)
		if err != nil {
			log.Printf("[WARN] scheduler: failed to run job %s -- %v", key.Key, err)
		}
	}
}

func (scheduler *scheduler) run() {
	for {
		select {
		case <-scheduler.ctx.Done():
			return
		case <-time.After(1 * time.Minute):
			scheduler.evaluate()
		}
	}
}

var region string

func main() {
	flag.StringVar(&region, "region", "us-west-2", "Aws Region")
	flag.Parse()

	var consulClient *consul.Client
	var ecsClient *ecs.ECS

	for {
		var err error
		consulClient, err = consul.NewClient(consul.DefaultConfig())
		leader, err := consulClient.Status().Leader()
		if err == nil {
			log.Printf("[INFO] main: connected to consul, leader is %s", leader)
			break
		}

		log.Printf("[WARN] main: could not connect to consul -- %v", err)
		time.Sleep(3 * time.Second)
	}

	for {
		sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
		if err == nil {
			ecsClient = ecs.New(sess)
			log.Println("[INFO] main: connected to aws")
			break
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	sched := &scheduler{
		ctx: ctx,
		kv:  consulClient.KV(),
		ecs: ecsClient,
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
