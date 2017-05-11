package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	consul "github.com/hashicorp/consul/api"
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
		check.HTTP = strings.Replace(arg, "${service-port}", servicePort, 1)
	case "tcp":
		check.TCP = strings.Replace(arg, "${service-port}", servicePort, 1)
	default:
		return nil
	}
	return consul.AgentServiceChecks{check}
}

func getServiceTags(container types.ContainerJSON) []string {
	return append([]string{"autoreg"}, strings.Split(container.Config.Labels[ServiceTagsKey], ",")...)
}

// Get the service host port.
func getServicePort(container types.ContainerJSON) string {
	servicePortId := container.Config.Labels[ServicePortKey]

	ports, ok := container.HostConfig.PortBindings[nat.Port(servicePortId)]
	if !ok || len(ports) == 0 {
		return ""
	}
	return ports[0].HostPort
}

func getServiceName(container types.ContainerJSON) string {
	for _, choice := range ServiceNameKeys {
		if name, ok := container.Config.Labels[choice]; ok {
			return name
		}
	}
	return container.Name
}

type registrator struct {
	ctx     context.Context
	kv      *consul.KV
	agent   *consul.Agent
	catalog *consul.Catalog
	docker  *docker.Client
}

func (a *registrator) stop(container types.ContainerJSON) error {
	return a.docker.ContainerStop(a.ctx, container.ID, nil)
}

func (a *registrator) deregister(container types.ContainerJSON) error {
	return a.agent.ServiceDeregister(container.ID)
}

func (a *registrator) register(container types.ContainerJSON) error {
	var port int
	consulChecks := getHealthChecks(container)
	servicePort := getServicePort(container)
	if servicePort != "" {
		port, _ = strconv.Atoi(servicePort)
	}

	return a.agent.ServiceRegister(&consul.AgentServiceRegistration{
		ID:     container.ID,
		Name:   getServiceName(container),
		Port:   port,
		Checks: consulChecks,
		Tags:   getServiceTags(container),
	})
}

func (a *registrator) consulIsRegistered(containerId string) (bool, error) {
	services, err := a.agent.Services()
	if err != nil {
		return false, err
	}

	for _, svc := range services {
		if svc.ID == containerId {
			return true, nil
		}
	}
	return false, nil
}

func (a *registrator) consulIsRunning(containerId string) (bool, error) {
	checks, err := a.agent.Checks()
	if err != nil {
		return false, err
	}

	for _, check := range checks {
		if check.ServiceID == containerId && check.Status == "critical" {
			return false, nil
		}
	}
	// always default to healthy
	return true, nil
}

func (a *registrator) isValidContainer(container types.ContainerJSON) bool {
	servicePort := getServicePort(container)
	return servicePort != ""
}

func (a *registrator) evaluate(containerId string) {
	container, err := a.docker.ContainerInspect(a.ctx, containerId)
	if err != nil {
		log.Printf("[WARN] registrator: failed to inspect -- %v", err)
		return
	}

	dockerRunning := container.State.Running
	consulHealthy, err := a.consulIsRunning(containerId)
	if err != nil {
		log.Printf("[WARN] registrator: failed to get health -- %v", err)
		return
	}

	consulRegistered, err := a.consulIsRegistered(containerId)
	if err != nil {
		log.Printf("[WARN] registrator: failed to get consul status -- %v", err)
		return
	}

	if dockerRunning && !consulRegistered {
		log.Printf("[DEBU] registrator: container is running, registering in consul %s", containerId)
		err := a.register(container)
		if err != nil {
			log.Printf("[ERRO] registrator: failed to registrator -- %v", err)
		}
		return
	}

	if dockerRunning && consulHealthy {
		log.Printf("[DEBU] registrator: container is healthy %s", containerId)
		return
	}

	if !dockerRunning && consulHealthy {
		log.Printf("[DEBU] registrator: container is not running, removing from consul %s", containerId)
		err := a.deregister(container)
		if err != nil {
			log.Printf("[ERRO] registrator: failed to deregistrator -- %v", err)
		}
		return
	}

	if dockerRunning && !consulHealthy {
		log.Printf("[DEBU] registrator: container is not health, stopping container %s", containerId)
		err := a.stop(container)
		if err != nil {
			log.Printf("[ERRO] registrator: failed to stop -- %v", err)
		}
		return
	}
}

func (a *registrator) run() {
	for {
		a.watch()

		// in case of watch errors, restart
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (a *registrator) watch() {
	messages, errs := a.docker.Events(a.ctx, types.EventsOptions{})

	for {
		select {
		case msg := <-messages:
			fmt.Printf("msg: %+v\n", msg)
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
			return
		case <-time.After(5 * time.Second):
			containers, err := a.docker.ContainerList(a.ctx, types.ContainerListOptions{})
			if err != nil {
				log.Printf("[WARN] registrator: failed to list containers -- %v", err)
				return
			}

			for _, container := range containers {
				a.evaluate(container.ID)
			}
		}
	}
}
