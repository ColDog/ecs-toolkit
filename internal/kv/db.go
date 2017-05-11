package kv

import (
	"encoding/json"
	"fmt"
	"sync"
)

type DB interface {
	Put(class string, key string, i interface{}) error
	Get(class string, key string, i interface{}) error
	Del(class string, key string) error
	Keys(class string) ([]string, error)
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

func (db *LocalDB) Keys(class string) ([]string, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()
	keys := []string{}
	for k := range db.data[class] {
		keys = append(keys, k)
	}
	return keys, nil
}

func (db *LocalDB) Put(class, key string, i interface{}) error {
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

func (db *LocalDB) Get(class, key string, i interface{}) error {
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

func (db *LocalDB) Del(class, key string) error {
	db.lock.Lock()
	defer db.lock.Unlock()
	inner := db.data[class]
	if inner == nil {
		return nil
	}
	delete(inner, key)
	return nil
}
