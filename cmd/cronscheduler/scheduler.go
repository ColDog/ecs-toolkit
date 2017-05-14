package main

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/coldog/tool-ecs/internal/kv"
	"github.com/gorhill/cronexpr"
	"github.com/pkg/errors"
	"log"
	"time"
)

var GetTime = func() time.Time { return time.Now() }

const CronJobType = "CronJob"

// A cron job represents a runnable cron job
type CronJob struct {
	// The last run executed by this job, used to find the next run.
	LastRun time.Time

	// Task definition ID to run and the cluster that it should run in.
	TaskDefinitionID string
	Cluster          string

	// The number of tasks to run.
	Replicas int

	// ECS Container overrides to apply.
	Overrides []*ecs.ContainerOverride

	// A Cron string.
	Schedule string
}

func (job *CronJob) Next() (time.Time, error) {
	if job.LastRun.IsZero() {
		job.LastRun = GetTime()
	}
	expr, err := cronexpr.Parse(job.Schedule)
	if err != nil {
		return time.Time{}, err
	}
	next := expr.Next(job.LastRun)
	return next, nil
}

func (job *CronJob) ShouldRun() (bool, error) {
	next, err := job.Next()
	if err != nil {
		return false, err
	}
	return GetTime().After(next), nil
}

type scheduler struct {
	ctx context.Context
	kv  kv.DB
	ecs ECSClient
}

func (scheduler *scheduler) runJob(key string, job *CronJob) error {
	shouldRun, err := job.ShouldRun()
	if err != nil {
		return err
	}
	if !shouldRun {
		return nil
	}

	log.Printf("[INFO] scheduler: running task %s/%s", job.Cluster, job.TaskDefinitionID)

	err = scheduler.ecs.RunTask(scheduler.ctx, &ecs.RunTaskInput{
		Cluster:        aws.String(job.Cluster),
		TaskDefinition: aws.String(job.TaskDefinitionID),
		StartedBy:      aws.String("CronScheduler"),
		Count:          aws.Int64(int64(job.Replicas)),
		Overrides: &ecs.TaskOverride{
			ContainerOverrides: job.Overrides,
		},
	})
	if err != nil {
		return errors.Wrap(err, "failed to run job")
	}

	job.LastRun = GetTime()
	err = scheduler.kv.Put(scheduler.ctx, CronJobType, key, job)
	if err != nil {
		return errors.Wrap(err, "failed to update timestamp")
	}

	return nil
}

func (scheduler *scheduler) evaluate() {
	keys, err := scheduler.kv.Keys(scheduler.ctx, CronJobType)
	if err != nil {
		log.Printf("[WARN] scheduler: failed to read keys -- %v", err)
		return
	}

	for _, key := range keys {
		job := &CronJob{}
		err := scheduler.kv.Get(scheduler.ctx, CronJobType, key, job)
		if err != nil {
			log.Printf("[WARN] scheduler: failed to run job %s -- %v", key, err)
			continue
		}

		log.Printf("[DEBU] scheduler: evaluating job %+v", job)
		err = scheduler.runJob(key, job)
		if err != nil {
			log.Printf("[WARN] scheduler: failed to run job %s -- %v", key, err)
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
