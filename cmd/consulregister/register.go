package main

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
	consul "github.com/hashicorp/consul/api"
	"golang.org/x/net/context"
	"log"
	"strconv"
	"strings"
	"time"
)

var (
	HealthCheckKey  = "service.health-check"
	ServicePortKey  = "service.port"
	ServiceTagsKey  = "service.tags"
	ServiceNameKeys = []string{
		"service.name",
		"com.amazonaws.ecs.task-definition-family",
	}
)

type Args []string

func (a Args) Get(i int) string {
	if len(a) <= i {
		return ""
	}
	return a[i]
}

// Format: [TYPE] [ARG] [Interval] [Timeout]
// Types:
//   - Script
//   - Shell
//   - HTTP
//   - TCP
//   - TTL
func getHealthChecks(container types.ContainerJSON) consul.AgentServiceChecks {
	checkDesc := container.Config.Labels[HealthCheckKey]
	servicePort := getServicePort(container)

	if checkDesc == "" {
		return nil
	}
	args := Args(strings.Split(checkDesc, " "))
	kind := args.Get(0)
	arg := args.Get(1)
	interval := args.Get(2)
	timeout := args.Get(3)

	check := &consul.AgentServiceCheck{
		Interval: interval,
		Timeout:  timeout,
	}

	switch strings.ToLower(kind) {
	case "script":
		check.Script = arg
	case "shell":
		check.Shell = arg
	case "http":
		check.HTTP = strings.Replace(arg, "${service.port}", servicePort, 1)
	case "tcp":
		check.TCP = strings.Replace(arg, "${service.port}", servicePort, 1)
	default:
		return nil
	}

	return consul.AgentServiceChecks{check}
}

func getServiceTags(container types.ContainerJSON) []string {
	return strings.Split(container.Config.Labels[ServiceTagsKey], ",")
}

// Get the service host port.
func getServicePort(container types.ContainerJSON) string {
	servicePortId := container.Config.Labels[ServicePortKey]

	ports, ok := container.HostConfig.PortBindings[nat.Port(servicePortId)]
	if !ok || len(ports) == 0 {
		return servicePortId
	}
	return ports[0].HostPort
}

// Get a key from the docker labels.
func getServiceName(container types.ContainerJSON) string {
	for _, choice := range ServiceNameKeys {
		if name, ok := container.Config.Labels[choice]; ok {
			return name
		}
	}
	return container.Name
}

type registrator struct {
	ctx    context.Context
	consul ConsulClient
	docker DockerClient
}

func (a *registrator) stop(id string, container types.ContainerJSON) error {
	return a.docker.ContainerStop(a.ctx, container.ID, nil)
}

func (a *registrator) deregister(id string) error {
	return a.consul.ServiceDeregister(id)
}

func (a *registrator) register(id string, container types.ContainerJSON) error {
	var port int
	consulChecks := getHealthChecks(container)
	servicePort := getServicePort(container)
	if servicePort != "" {
		port, _ = strconv.Atoi(servicePort)
	}

	service := &consul.AgentServiceRegistration{
		ID:     id,
		Name:   getServiceName(container),
		Port:   port,
		Checks: consulChecks,
		Tags:   getServiceTags(container),
	}

	log.Printf("[DEBU] registrator: register %+v", service)
	return a.consul.ServiceRegister(service)
}

func (a *registrator) evaluate(containerId string) {
	container, err := a.docker.ContainerInspect(a.ctx, containerId)
	if err != nil {
		log.Printf("[WARN] registrator: failed to inspect -- %v", err)
		return
	}

	if getServiceName(container) == "" {
		// No service name means we don't care
		return
	}

	id := containerId[:12]

	dockerRunning := container.State.Running
	consulHealthy, err := a.consul.ServiceIsRunning(id)
	if err != nil {
		log.Printf("[WARN] registrator: failed to get health -- %v", err)
		return
	}

	consulRegistered, err := a.consul.ServiceIsRegistered(id)
	if err != nil {
		log.Printf("[WARN] registrator: failed to get consul status -- %v", err)
		return
	}

	if dockerRunning && !consulRegistered {
		log.Printf("[DEBU] registrator: container is running, registering in consul %s", id)
		err := a.register(id, container)
		if err != nil {
			log.Printf("[ERRO] registrator: failed to registrator -- %v", err)
		}
		return
	}

	if !dockerRunning && consulRegistered {
		log.Printf("[DEBU] registrator: container is not running, removing from consul %s", id)
		err := a.deregister(id)
		if err != nil {
			log.Printf("[ERRO] registrator: failed to deregistrator -- %v", err)
		}
		return
	}

	if dockerRunning && !consulHealthy {
		log.Printf("[DEBU] registrator: container is not healthy, stopping container and deregistering %s", id)
		err := a.stop(id, container)
		if err != nil {
			log.Printf("[ERRO] registrator: failed to stop -- %v", err)
		}
		err = a.deregister(id)
		if err != nil {
			log.Printf("[ERRO] registrator: failed to deregister -- %v", err)
		}
		return
	}
}

func (a *registrator) run() {
	messages, errs := a.docker.Events(a.ctx, types.EventsOptions{})

	for {
		select {
		case msg := <-messages:
			if msg.Type != "container" {
				continue
			}
			switch msg.Action {
			case "create":
				a.evaluate(msg.ID)
			case "stop", "kill":
				a.evaluate(msg.ID)
			}
		case err := <-errs:
			log.Printf("[WARN] registrator: recieved error from docker events -- %v", err)
			messages, errs = a.docker.Events(a.ctx, types.EventsOptions{})
		case <-time.After(5 * time.Second):
			containers, err := a.docker.ContainerList(a.ctx, types.ContainerListOptions{})
			if err != nil {
				log.Printf("[WARN] registrator: failed to list containers -- %v", err)
				return
			}

			for _, container := range containers {
				a.evaluate(container.ID)
			}
		case <-a.ctx.Done():
			return
		}
	}
}
