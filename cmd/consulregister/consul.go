package main

import (
	consul "github.com/hashicorp/consul/api"
)

type ConsulClient interface {
	ServiceDeregister(string) error
	ServiceRegister(*consul.AgentServiceRegistration) error
	ServiceIsRunning(string) (bool, error)
	ServiceIsRegistered(string) (bool, error)
}

func NewConsulClient(client *consul.Client) ConsulClient {
	return &consulClient{client.Agent()}
}

type consulClient struct {
	*consul.Agent
}

func (client *consulClient) ServiceIsRunning(id string) (bool, error) {
	checks, err := client.Agent.Checks()
	if err != nil {
		return false, err
	}

	for _, check := range checks {
		if check.ServiceID == id && check.Status == "critical" {
			return false, nil
		}
	}
	return true, nil
}

func (client *consulClient) ServiceIsRegistered(id string) (bool, error) {
	services, err := client.Agent.Services()
	if err != nil {
		return false, err
	}

	for _, svc := range services {
		if svc.ID == id {
			return true, nil
		}
	}
	return false, nil
}
