package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
)

var region string

func main() {
	flag.StringVar(&region, "region", "us-west-2", "Aws Region")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())

	sched := &scheduler{
		ctx: ctx,
		kv:  NewConsulKVClient(),
		ecs: NewECSClient(),
	}

	sigs := make(chan os.Signal)
	signal.Notify(sigs, os.Interrupt, os.Kill)

	go func() {
		sig := <-sigs
		log.Printf("[WARN] main: exiting due to %v", sig)
		cancel()
		os.Exit(0)
	}()

	err := sched.kv.Open(ctx)
	if err != nil {
		log.Fatalf("[FATA] main: failed to open kv -- %v", err)
	}

	err = sched.ecs.Open(ctx)
	if err != nil {
		log.Fatalf("[FATA] main: failed to open ecs -- %v", err)
	}

	sched.run()
}
