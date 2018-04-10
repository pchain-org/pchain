package types

import (
	"errors"
	"fmt"
)

type KeyValue struct {
	Key string
	Value interface{}
}

type KeyValueSet struct {
	KVArray []KeyValue
}

func MakeKeyValueSet() KeyValueSet{

	return KeyValueSet{KVArray: make([]KeyValue, 0)}
}

func (kv *KeyValueSet) Size() int {
	return len(kv.KVArray)
}

func (kv *KeyValueSet) Set(key string, value interface{}) {

	for index, item := range kv.KVArray {

		if item.Key == key {
			kv.KVArray[index].Value = value
			return
		}
	}

	kv.KVArray = append(kv.KVArray, KeyValue{Key:key, Value:value})
}

func (kv *KeyValueSet) Get(key string) (interface{}, error) {

	for _, item := range kv.KVArray {

		if item.Key == key {
			return item.Value, nil
		}
	}

	return nil, errors.New("Value not exist")
}

func (kv *KeyValueSet) GetByIndex(index int) (string, interface{}, error) {

	if index > kv.Size() - 1 {
		return "", nil, errors.New("KeyValue out of range")
	}

	item := kv.KVArray[index]

	return item.Key, item.Value, nil
}

func (kv *KeyValueSet) String() string {

	ret := "{"

	for i, item := range kv.KVArray {
		ret += fmt.Sprintf("\"%s\":\"%v\"", item.Key, item.Value)
		if i != len(kv.KVArray) - 1 {
			ret += ","
		}
	}

	ret += "}"

	return ret
}