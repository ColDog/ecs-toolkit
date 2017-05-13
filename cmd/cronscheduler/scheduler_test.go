package main

import (
	"context"
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
func (m *MockECS) RunTask(cluster, taskDefinition string) error {
	return m.Called(cluster, taskDefinition).Error(0)
}

type MockConsul struct {
	mock.Mock
}

func (m *MockConsul) Open(ctx context.Context) error        { return nil }
func (m *MockConsul) Update(key string, value []byte) error { return m.Called(key).Error(0) }
func (m *MockConsul) List(prefix string) (map[string][]byte, error) {
	args := m.Called(prefix)
	return args.Get(0).(map[string][]byte), args.Error(1)
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
	mockConsul := &MockConsul{}
	ctx := context.Background()
	sched := &scheduler{
		ctx: ctx,
		ecs: mockEcs,
		kv:  mockConsul,
	}
	jobs := map[string][]byte{
		"job1": []byte(`{
		    "LastRun": "2017-05-04T00:00:00Z",
		    "TaskDefinitionID": "testTask",
		    "Cluster": "testCluster",
		    "Schedule": "0 * * * *"
		}`),
	}
	mockConsul.On("List", "cronjobs/").Return(jobs, nil)
	mockConsul.On("Update", "cronjobs/job1").Return(nil)
	mockEcs.On("RunTask", "testCluster", "testTask").Return(nil)

	sched.evaluate()

	mockConsul.AssertCalled(t, "List", "cronjobs/")
	mockConsul.AssertCalled(t, "Update", "cronjobs/job1")
	mockEcs.AssertCalled(t, "RunTask", "testCluster", "testTask")
}

func TestScheduler_DoNotStart(t *testing.T) {
	mockEcs := &MockECS{}
	mockConsul := &MockConsul{}
	ctx := context.Background()
	sched := &scheduler{
		ctx: ctx,
		ecs: mockEcs,
		kv:  mockConsul,
	}
	jobs := map[string][]byte{
		// LastRun = current time
		"job1": []byte(`{
		    "LastRun": "2017-05-05T00:00:00Z",
		    "TaskDefinitionID": "testTask",
		    "Cluster": "testCluster",
		    "Schedule": "0 * * * *"
		}`),
	}
	mockConsul.On("List", "cronjobs/").Return(jobs, nil)
	mockEcs.On("RunTask", "testCluster", "testTask").Return(nil)

	sched.evaluate()

	mockConsul.AssertCalled(t, "List", "cronjobs/")
	mockEcs.AssertNotCalled(t, "RunTask", "testCluster", "testTask")
}
