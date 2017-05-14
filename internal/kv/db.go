package kv

import (
	"encoding/json"
	"sync"
	"fmt"
	"context"
)

type DB interface {
	Put(ctx context.Context, class string, key string, i interface{}) error
	Get(ctx context.Context, class string, key string, i interface{}) error
	Del(ctx context.Context, class string, key string) error
	Keys(ctx context.Context, class string) ([]string, error)
}

func NewLocalDB() *LocalDB {
	return &LocalDB{
		data: map[string]map[string][]byte{},
	}
}

type LocalDB struct {
	lock sync.RWMutex
	data map[string]map[string][]byte
}

func (db *LocalDB) Keys(ctx context.Context, class string) ([]string, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()
	keys := []string{}
	for k := range db.data[class] {
		keys = append(keys, k)
	}
	return keys, nil
}

func (db *LocalDB) Put(ctx context.Context, class, key string, i interface{}) error {
	db.lock.RLock()
	defer db.lock.RUnlock()
	data, err := json.Marshal(i)
	if err != nil {
		return err
	}
	if db.data[class] == nil {
		db.data[class] = map[string][]byte{}
	}
	db.data[class][key] = data
	return nil
}

func (db *LocalDB) Get(ctx context.Context, class, key string, i interface{}) error {
	db.lock.Lock()
	defer db.lock.Unlock()
	inner := db.data[class]
	if inner == nil {
		return fmt.Errorf("ErrNotFound: %s(%s)", class, key)
	}
	err := json.Unmarshal(inner[key], i)
	if err != nil {
		return err
	}
	return nil
}

func (db *LocalDB) Del(ctx context.Context, class, key string) error {
	db.lock.Lock()
	defer db.lock.Unlock()
	inner := db.data[class]
	if inner == nil {
		return nil
	}
	delete(inner, key)
	return nil
}
