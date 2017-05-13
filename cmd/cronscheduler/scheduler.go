package main

import (
	"context"
	"encoding/json"
	"github.com/gorhill/cronexpr"
	"github.com/pkg/errors"
	"log"
	"time"
)

var GetTime = func() time.Time { return time.Now() }

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
	kv  ConsulKVClient
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

	err = scheduler.ecs.RunTask(job.Cluster, job.TaskDefinitionID)
	if err != nil {
		return errors.Wrap(err, "failed to run job")
	}

	job.LastRun = GetTime()
	data, err := json.Marshal(job)
	if err != nil {
		return errors.Wrap(err, "failed to marshal job")
	}

	err = scheduler.kv.Update(KeyPrefix+key, data)
	if err != nil {
		return errors.Wrap(err, "failed to update timestamp")
	}

	return nil
}

func (scheduler *scheduler) evaluate() {
	values, err := scheduler.kv.List(KeyPrefix)
	if err != nil {
		log.Printf("[WARN] scheduler: failed to read keys -- %v", err)
		return
	}

	for key, val := range values {
		job := &CronJob{}
		err := json.Unmarshal(val, job)
		if err != nil {
			log.Printf("[WARN] scheduler: failed to read cron job at %s -- %v", key, err)
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
