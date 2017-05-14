package main

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/coldog/tool-ecs/internal/kv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

func init() {
	// fixed time to "2017-05-05T00:00:00Z"
	GetTime = func() time.Time {
		return time.Date(2017, 05, 05, 0, 0, 0, 0, time.UTC)
	}
}

type MockECS struct {
	mock.Mock
}

func (m *MockECS) Open(ctx context.Context) error { return nil }
func (m *MockECS) RunTask(ctx context.Context, input *ecs.RunTaskInput) error {
	return m.Called(input).Error(0)
}

func TestCronJob_Next(t *testing.T) {
	job := &CronJob{
		LastRun:          time.Date(2017, 05, 05, 0, 0, 0, 0, time.UTC),
		Cluster:          "default",
		TaskDefinitionID: "test",
		Schedule:         "0 * * * *", // at minute zero
	}
	next, err := job.Next()
	assert.Nil(t, err)
	assert.Equal(t, "2017-05-05 01:00:00 +0000 UTC", next.String())
}

func TestCronJob_ShouldRun(t *testing.T) {
	job := &CronJob{
		LastRun:          time.Date(2017, 05, 04, 0, 0, 0, 0, time.UTC),
		Cluster:          "default",
		TaskDefinitionID: "test",
		Schedule:         "0 * * * *", // at minute zero
	}
	run, err := job.ShouldRun()
	assert.Nil(t, err)
	assert.True(t, run)
}

func TestCronJob_ShouldNotRun(t *testing.T) {
	job := &CronJob{
		LastRun:          time.Now(), // already run
		Cluster:          "default",
		TaskDefinitionID: "test",
		Schedule:         "0 * * * *", // at minute zero
	}
	run, err := job.ShouldRun()
	assert.Nil(t, err)
	assert.False(t, run)
}

func TestScheduler_Start(t *testing.T) {
	mockEcs := &MockECS{}
	ctx := context.Background()
	sched := &scheduler{
		ctx: ctx,
		ecs: mockEcs,
		kv:  kv.NewLocalDB(),
	}
	sched.kv.Put(context.Background(), CronJobType, "job1", &CronJob{
		LastRun:          time.Date(2017, 05, 04, 0, 0, 0, 0, time.UTC),
		TaskDefinitionID: "testTask",
		Cluster:          "testCluster",
		Schedule:         "0 * * * *",
		Replicas:         5,
	})
	input := &ecs.RunTaskInput{
		Count:          aws.Int64(5),
		StartedBy:      aws.String("CronScheduler"),
		TaskDefinition: aws.String("testTask"),
		Cluster:        aws.String("testCluster"),
		Overrides:      &ecs.TaskOverride{},
	}
	mockEcs.On("RunTask", input).Return(nil)

	sched.evaluate()

	mockEcs.AssertCalled(t, "RunTask", input)
}

func TestScheduler_DoNotStart(t *testing.T) {
	mockEcs := &MockECS{}
	ctx := context.Background()
	sched := &scheduler{
		ctx: ctx,
		ecs: mockEcs,
		kv:  kv.NewLocalDB(),
	}
	sched.kv.Put(context.Background(), CronJobType, "job1", &CronJob{
		LastRun:          GetTime(),
		TaskDefinitionID: "testTask",
		Cluster:          "testCluster",
		Schedule:         "0 * * * *",
	})
	mockEcs.On("RunTask", "testCluster", "testTask").Return(nil)

	sched.evaluate()

	mockEcs.AssertNotCalled(t, "RunTask")
}
