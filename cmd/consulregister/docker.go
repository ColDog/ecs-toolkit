package main

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"golang.org/x/net/context"
	"time"
)

type DockerClient interface {
	ContainerStop(context.Context, string, *time.Duration) error
	ContainerInspect(context.Context, string) (types.ContainerJSON, error)
	Events(context.Context, types.EventsOptions) (<-chan events.Message, <-chan error)
	ContainerList(context.Context, types.ContainerListOptions) ([]types.Container, error)
}
