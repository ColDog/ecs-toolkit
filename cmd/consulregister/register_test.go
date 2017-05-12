package main

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/go-connections/nat"
	consul "github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"
	"testing"
	"time"
)

type MockConsul struct {
	mock.Mock
}

func (m *MockConsul) ServiceDeregister(id string) error {
	return m.Called(id).Error(0)
}

func (m *MockConsul) ServiceRegister(svc *consul.AgentServiceRegistration) error {
	return m.Called().Error(0)
}

func (m *MockConsul) ServiceIsRunning(id string) (bool, error) {
	args := m.Called(id)
	return args.Bool(0), args.Error(1)
}

func (m *MockConsul) ServiceIsRegistered(id string) (bool, error) {
	args := m.Called(id)
	return args.Bool(0), args.Error(1)
}

type MockDocker struct {
	mock.Mock
}

func (m *MockDocker) ContainerStop(ctx context.Context, id string, timeout *time.Duration) error {
	println("container stop", id)
	return m.Called(id).Error(0)
}

func (m *MockDocker) ContainerInspect(ctx context.Context, id string) (types.ContainerJSON, error) {
	args := m.Called(id)
	return args.Get(0).(types.ContainerJSON), args.Error(1)
}

func (m *MockDocker) Events(context.Context, types.EventsOptions) (<-chan events.Message, <-chan error) {
	return make(chan events.Message), make(chan error)
}

func (m *MockDocker) ContainerList(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
	args := m.Called(opts)
	return args.Get(0).([]types.Container), args.Error(1)
}

func TestLabels_GetName(t *testing.T) {
	spec := types.ContainerJSON{
		Config: &container.Config{
			Labels: map[string]string{
				"service.name": "testing123",
			},
		},
		ContainerJSONBase: &types.ContainerJSONBase{
			HostConfig: &container.HostConfig{},
		},
	}

	name := getServiceName(spec)

	assert.Equal(t, "testing123", name)
}

func TestLabels_GetChecks(t *testing.T) {
	spec := types.ContainerJSON{
		Config: &container.Config{
			Labels: map[string]string{
				"service.health-check": "TCP 127.0.0.1:3000 20s 30s",
			},
		},
		ContainerJSONBase: &types.ContainerJSONBase{
			HostConfig: &container.HostConfig{},
		},
	}

	checks := getHealthChecks(spec)

	assert.Equal(t, "30s", checks[0].Timeout)
	assert.Equal(t, "20s", checks[0].Interval)
	assert.Equal(t, "127.0.0.1:3000", checks[0].TCP)
}

func TestLabels_GetPort(t *testing.T) {
	spec := types.ContainerJSON{
		Config: &container.Config{
			Labels: map[string]string{
				"service.port": "80/tcp",
			},
		},
		ContainerJSONBase: &types.ContainerJSONBase{
			HostConfig: &container.HostConfig{
				PortBindings: nat.PortMap{
					"80/tcp": []nat.PortBinding{
						{HostPort: "3000"},
					},
				},
			},
		},
	}

	port := getServicePort(spec)

	assert.Equal(t, "3000", port)
}

func TestLabels_PortHealthCheck(t *testing.T) {
	spec := types.ContainerJSON{
		Config: &container.Config{
			Labels: map[string]string{
				"service.port":         "80/tcp",
				"service.health-check": "HTTP 127.0.0.1:${service.port} 20s 30s",
			},
		},
		ContainerJSONBase: &types.ContainerJSONBase{
			HostConfig: &container.HostConfig{
				PortBindings: nat.PortMap{
					"80/tcp": []nat.PortBinding{
						{HostPort: "3000"},
					},
				},
			},
		},
	}

	checks := getHealthChecks(spec)

	assert.Equal(t, "30s", checks[0].Timeout)
	assert.Equal(t, "20s", checks[0].Interval)
	assert.Equal(t, "127.0.0.1:3000", checks[0].HTTP)
}

func TestRegister_RunningNotRegistered(t *testing.T) {
	spec := types.ContainerJSON{
		Config: &container.Config{
			Labels: map[string]string{
				"service.name":         "testing123",
				"service.port":         "80/tcp",
				"service.health-check": "HTTP 127.0.0.1:${service.port} 20s 30s",
			},
		},
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e",
			HostConfig: &container.HostConfig{
				PortBindings: nat.PortMap{
					"80/tcp": []nat.PortBinding{
						{HostPort: "3000"},
					},
				},
			},
			State: &types.ContainerState{
				Running: true,
			},
		},
	}
	mockConsul := &MockConsul{}
	mockDocker := &MockDocker{}
	reg := &registrator{
		docker: mockDocker,
		consul: mockConsul,
	}
	mockDocker.On("ContainerInspect", "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e").Return(spec, nil)
	mockConsul.On("ServiceIsRunning", "a156e4885334").Return(false, nil)
	mockConsul.On("ServiceIsRegistered", "a156e4885334").Return(false, nil)
	mockConsul.On("ServiceRegister").Return(nil)

	reg.evaluate("a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e")

	mockConsul.AssertCalled(t, "ServiceRegister")
}

func TestRegister_NotRunningRegistered(t *testing.T) {
	spec := types.ContainerJSON{
		Config: &container.Config{
			Labels: map[string]string{
				"service.name":         "testing123",
				"service.port":         "80/tcp",
				"service.health-check": "HTTP 127.0.0.1:${service.port} 20s 30s",
			},
		},
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e",
			HostConfig: &container.HostConfig{
				PortBindings: nat.PortMap{
					"80/tcp": []nat.PortBinding{
						{HostPort: "3000"},
					},
				},
			},
			State: &types.ContainerState{
				Running: false,
			},
		},
	}
	mockConsul := &MockConsul{}
	mockDocker := &MockDocker{}
	reg := &registrator{
		docker: mockDocker,
		consul: mockConsul,
	}
	mockDocker.On("ContainerInspect", "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e").Return(spec, nil)
	mockConsul.On("ServiceIsRunning", "a156e4885334").Return(true, nil)
	mockConsul.On("ServiceIsRegistered", "a156e4885334").Return(true, nil)
	mockConsul.On("ServiceRegister").Return(nil)
	mockConsul.On("ServiceDeregister", "a156e4885334").Return(nil)

	reg.evaluate("a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e")

	mockConsul.AssertCalled(t, "ServiceDeregister", "a156e4885334")
}

func TestRegister_RunningNotHealthy(t *testing.T) {
	spec := types.ContainerJSON{
		Config: &container.Config{
			Labels: map[string]string{
				"service.name":         "testing123",
				"service.port":         "80/tcp",
				"service.health-check": "HTTP 127.0.0.1:${service.port} 20s 30s",
			},
		},
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e",
			HostConfig: &container.HostConfig{
				PortBindings: nat.PortMap{
					"80/tcp": []nat.PortBinding{
						{HostPort: "3000"},
					},
				},
			},
			State: &types.ContainerState{
				Running: true,
			},
		},
	}
	mockConsul := &MockConsul{}
	mockDocker := &MockDocker{}
	reg := &registrator{
		docker: mockDocker,
		consul: mockConsul,
	}
	mockDocker.On("ContainerInspect", "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e").Return(spec, nil)
	mockDocker.On("ContainerStop", "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e").Return(nil)
	mockConsul.On("ServiceIsRunning", "a156e4885334").Return(false, nil)
	mockConsul.On("ServiceIsRegistered", "a156e4885334").Return(true, nil)
	mockConsul.On("ServiceRegister").Return(nil)
	mockConsul.On("ServiceDeregister", "a156e4885334").Return(nil)

	reg.evaluate("a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e")

	mockConsul.AssertCalled(t, "ServiceDeregister", "a156e4885334")
	mockDocker.AssertCalled(t, "ContainerStop", "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e")
}

func TestRegister_RunningHealthy(t *testing.T) {
	spec := types.ContainerJSON{
		Config: &container.Config{
			Labels: map[string]string{
				"service.name":         "testing123",
				"service.port":         "80/tcp",
				"service.health-check": "HTTP 127.0.0.1:${service.port} 20s 30s",
			},
		},
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e",
			HostConfig: &container.HostConfig{
				PortBindings: nat.PortMap{
					"80/tcp": []nat.PortBinding{
						{HostPort: "3000"},
					},
				},
			},
			State: &types.ContainerState{
				Running: true,
			},
		},
	}
	mockConsul := &MockConsul{}
	mockDocker := &MockDocker{}
	reg := &registrator{
		docker: mockDocker,
		consul: mockConsul,
	}
	mockDocker.On("ContainerInspect", "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e").Return(spec, nil)
	mockDocker.On("ContainerStop", "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e").Return(nil)
	mockConsul.On("ServiceIsRunning", "a156e4885334").Return(true, nil)
	mockConsul.On("ServiceIsRegistered", "a156e4885334").Return(true, nil)
	mockConsul.On("ServiceRegister").Return(nil)
	mockConsul.On("ServiceDeregister", "a156e4885334").Return(nil)

	reg.evaluate("a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e")

	mockConsul.AssertNotCalled(t, "ServiceDeregister")
	mockDocker.AssertNotCalled(t, "ContainerStop")
}

func TestRegister_NotRunningNotRegistered(t *testing.T) {
	spec := types.ContainerJSON{
		Config: &container.Config{
			Labels: map[string]string{
				"service.name":         "testing123",
				"service.port":         "80/tcp",
				"service.health-check": "HTTP 127.0.0.1:${service.port} 20s 30s",
			},
		},
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e",
			HostConfig: &container.HostConfig{
				PortBindings: nat.PortMap{
					"80/tcp": []nat.PortBinding{
						{HostPort: "3000"},
					},
				},
			},
			State: &types.ContainerState{
				Running: false,
			},
		},
	}
	mockConsul := &MockConsul{}
	mockDocker := &MockDocker{}
	reg := &registrator{
		docker: mockDocker,
		consul: mockConsul,
	}
	mockDocker.On("ContainerInspect", "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e").Return(spec, nil)
	mockDocker.On("ContainerStop", "a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e").Return(nil)
	mockConsul.On("ServiceIsRunning", "a156e4885334").Return(false, nil)
	mockConsul.On("ServiceIsRegistered", "a156e4885334").Return(false, nil)
	mockConsul.On("ServiceRegister").Return(nil)
	mockConsul.On("ServiceDeregister", "a156e4885334").Return(nil)

	reg.evaluate("a156e48853345e590bb9fa05be0ce53505895ebc465e4977aaab1c5673d9db2e")

	mockConsul.AssertNotCalled(t, "ServiceDeregister")
	mockDocker.AssertNotCalled(t, "ContainerStop")
}
