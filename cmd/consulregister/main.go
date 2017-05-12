package main

import (
	"context"
	docker "github.com/docker/docker/client"
	consul "github.com/hashicorp/consul/api"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	var consulClient *consul.Client
	var dockerClient *docker.Client

	for {
		var err error
		consulClient, err = consul.NewClient(consul.DefaultConfig())
		leader, err := consulClient.Status().Leader()
		if err == nil {
			log.Printf("[INFO] main: connected to consul, leader is %s", leader)
			break
		}

		log.Printf("[WARN] main: could not connect to consul -- %v", err)
		time.Sleep(3 * time.Second)
	}

	for {
		var err error
		dockerClient, err = docker.NewEnvClient()
		_, err = dockerClient.Ping(context.Background())
		if err == nil {
			log.Println("[INFO] main: connected to docker")
			break
		}

		log.Printf("[WARN] main: could not connect to docker -- %v", err)
		time.Sleep(3 * time.Second)
	}

	ctx, cancel := context.WithCancel(context.Background())

	reg := &registrator{
		docker: dockerClient,
		consul: NewConsulClient(consulClient),
		ctx:    ctx,
	}

	sigs := make(chan os.Signal)
	signal.Notify(sigs, os.Interrupt, os.Kill)

	go func() {
		sig := <-sigs
		log.Printf("[WARN] main: exiting due to %v", sig)
		cancel()
		os.Exit(0)
	}()

	reg.run()
}
