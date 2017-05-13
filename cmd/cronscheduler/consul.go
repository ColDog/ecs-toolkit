package main

import (
	"context"
	consul "github.com/hashicorp/consul/api"
	"log"
	"time"
)

type ConsulKVClient interface {
	Open(ctx context.Context) error
	List(prefix string) (map[string][]byte, error)
	Update(key string, value []byte) error
}

func NewConsulKVClient() ConsulKVClient {
	return &consulKVClient{}
}

type consulKVClient struct {
	kv *consul.KV
}

func (client *consulKVClient) Open(ctx context.Context) error {
	for {
		consulClient, err := consul.NewClient(consul.DefaultConfig())
		leader, err := consulClient.Status().Leader()
		if err == nil {
			log.Printf("[INFO] main: connected to consul, leader is %s", leader)
			return nil
		}

		log.Printf("[WARN] main: could not connect to consul, trying again in 3 seconds -- %v", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func (client *consulKVClient) List(prefix string) (map[string][]byte, error) {
	pairs, _, err := client.kv.List(prefix, nil)
	if err != nil {
		return nil, err
	}

	out := map[string][]byte{}
	for _, pair := range pairs {
		out[pair.Key] = pair.Value
	}
	return out, nil
}

func (client *consulKVClient) Update(key string, data []byte) error {
	_, err := client.kv.Put(
		&consul.KVPair{
			Key:   key,
			Value: data,
		},
		&consul.WriteOptions{},
	)
	return err
}
