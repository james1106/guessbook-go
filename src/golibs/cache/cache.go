package safemap
// Package cache providers a cache management for baa.

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// Cacher a cache management for baa
type Cacher interface {
	// Exist return true if value cached by given key
	Exist(key string) bool
	// Get returns value to out by given key
	Get(key string, out interface{}) error
	// Set cache value by given key
	Set(key string, v interface{}, ttl int64) error
	// Incr increases cached int-type value by given key as a counter
	// if key not exist, before increase set value with zero
	Incr(key string) (int64, error)
	// Decr decreases cached int-type value by given key as a counter
	// if key not exist, before increase set value with zero
	// NOTE: memcached returns uint type cannot be less than zero
	Decr(key string) (int64, error)
	// Delete delete cached data by given key
	Delete(key string) error
	// Flush flush cacher
	Flush() error
	// Start new a cacher and start service
	Start(Options) error
}

// Item cache storage item
type Item struct {
	Val        interface{} // real object value
	TTL        int64       // cache life time
	Expiration int64       // expired time
}

// ItemBinary cache item encoded data
type ItemBinary []byte

// Options cache options
type Options struct {
	Name    string                 // cache name
	Adapter string                 // adapter
	Prefix  string                 // cache key prefix
	Config  map[string]interface{} // config for adapter
}

type instanceFunc func() Cacher

var adapters = make(map[string]instanceFunc)

// New create a Cacher
func New(o Options) Cacher {
	if o.Name == "" {
		o.Name = "_DEFAULT_"
	}
	if o.Adapter == "" {
		panic("cache.New: cannot use empty adapter")
	}
	c, err := NewCacher(o.Adapter, o)
	if err != nil {
		panic("cache.New: " + err.Error())
	}
	return c
}

// NewCacher creates and returns a new cacher by given adapter name and configuration.
// It panics when given adapter isn't registered and starts GC automatically.
func NewCacher(name string, o Options) (Cacher, error) {
	f, ok := adapters[name]
	if !ok {
		return nil, fmt.Errorf("cache: unknown adapter '%s'(forgot to import?)", name)
	}
	adapter := f()
	return adapter, adapter.Start(o)
}

// Register registers a adapter
func Register(name string, f instanceFunc) {
	if f == nil {
		panic("cache.Register: cannot register adapter with nil func")
	}
	if _, ok := adapters[name]; ok {
		panic(fmt.Errorf("cache.Register: cannot register adapter '%s' twice", name))
	}
	adapters[name] = f
}

// NewItem create a cache item
func NewItem(val interface{}, ttl int64) *Item {
	item := &Item{Val: val, TTL: ttl}
	if ttl > 0 {
		item.Expiration = time.Now().Add(time.Duration(ttl) * time.Second).UnixNano()
	}
	return item
}

// Expired check item has expired
func (t *Item) Expired() bool {
	return t.TTL > 0 && time.Now().UnixNano() >= t.Expiration
}

// Incr increases given value
func (t *Item) Incr() error {
	switch t.Val.(type) {
	case int, int8, int16, int32, int64:
		t.Val = reflect.ValueOf(t.Val).Int() + 1
	case uint, uint8, uint16, uint32, uint64:
		t.Val = int64(reflect.ValueOf(t.Val).Uint()) + 1
	default:
		return fmt.Errorf("item value is not int-type")
	}
	return nil
}

// Decr decreases given value
func (t *Item) Decr() error {
	switch t.Val.(type) {
	case int, int8, int16, int32, int64:
		t.Val = reflect.ValueOf(t.Val).Int() - 1
	case uint, uint8, uint16, uint32, uint64:
		t.Val = int64(reflect.ValueOf(t.Val).Uint()) - 1
	default:
		return fmt.Errorf("item value is not int-type")
	}
	return nil
}

// Encode encode item to bytes by gob
func (t *Item) Encode() (ItemBinary, error) {
	buf := bytes.NewBuffer(nil)
	err := gob.NewEncoder(buf).Encode(t)
	return buf.Bytes(), err
}

// Decode item value to out interface
func (t *Item) Decode(out interface{}) error {
	rv := reflect.ValueOf(out)
	if rv.IsNil() {
		return fmt.Errorf("cache: out is nil")
	}
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("cache: out must be a pointer")
	}
	for rv.Kind() == reflect.Ptr {
		if !rv.Elem().IsValid() && rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	if !rv.CanSet() {
		return fmt.Errorf("cache: out cannot set value")
	}
	rt := reflect.ValueOf(t.Val)
	if rv.Type() != rt.Type() {
		return fmt.Errorf("cache: out is different type with stored value %v, %v", rv.Type(), rt.Type())
	}
	rv.Set(rt)
	return nil
}

// Item decode bytes data to cache item use gob
func (t ItemBinary) Item() (*Item, error) {
	buf := bytes.NewBuffer(t)
	item := new(Item)
	err := gob.NewDecoder(buf).Decode(&item)
	return item, err
}

// SimpleType check value type is simple type or not
func SimpleType(v interface{}) bool {
	switch v.(type) {
	case string:
		return true
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32, float64:
		return true
	case bool:
		return true
	default:
		return false
	}
}

// SimpleValue return value to output with type convert
func SimpleValue(v []byte, o interface{}) bool {
	switch o.(type) {
	case *string:
		*o.(*string) = string(v)
	case *bool:
		*o.(*bool), _ = strconv.ParseBool(string(v))
	case *int:
		t, _ := strconv.ParseInt(string(v), 10, 64)
		*o.(*int) = int(t)
	case *int8:
		t, _ := strconv.ParseInt(string(v), 10, 64)
		*o.(*int8) = int8(t)
	case *int16:
		t, _ := strconv.ParseInt(string(v), 10, 64)
		*o.(*int16) = int16(t)
	case *int32:
		t, _ := strconv.ParseInt(string(v), 10, 64)
		*o.(*int32) = int32(t)
	case *int64:
		*o.(*int64), _ = strconv.ParseInt(string(v), 10, 64)
	case *uint:
		t, _ := strconv.ParseUint(string(v), 10, 64)
		*o.(*uint) = uint(t)
	case *uint8:
		t, _ := strconv.ParseUint(string(v), 10, 64)
		*o.(*uint8) = uint8(t)
	case *uint16:
		t, _ := strconv.ParseUint(string(v), 10, 64)
		*o.(*uint16) = uint16(t)
	case *uint32:
		t, _ := strconv.ParseUint(string(v), 10, 64)
		*o.(*uint32) = uint32(t)
	case *uint64:
		*o.(*uint64), _ = strconv.ParseUint(string(v), 10, 64)
	case *float32:
		t, _ := strconv.ParseFloat(string(v), 64)
		*o.(*float32) = float32(t)
	case *float64:
		*o.(*float64), _ = strconv.ParseFloat(string(v), 64)
	default:
		return false
	}
	return true
}

func init() {
	gob.Register(time.Time{})
	gob.Register(&Item{})
}
