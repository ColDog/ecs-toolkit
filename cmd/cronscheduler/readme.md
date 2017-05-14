# CronScheduler

The cron scheduler schedules task definitions to run at a specific time.

## Spec

The following shows the Go Spec for a CronJob.

```go
// A cron job represents a runnable cron job.
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

// The overrides that should be sent to a container.
// Please also see https://docs.aws.amazon.com/goto/WebAPI/ecs-2014-11-13/ContainerOverride
type ContainerOverride struct {
	// The command to send to the container that overrides the default command from
	// the Docker image or the task definition.
	Command []*string `locationName:"command" type:"list"`

	// The environment variables to send to the container. You can add new environment
	// variables, which are added to the container at launch, or you can override
	// the existing environment variables from the Docker image or the task definition.
	Environment []*KeyValuePair `locationName:"environment" type:"list"`

	// The name of the container that receives the override.
	Name *string `locationName:"name" type:"string"`
}
```