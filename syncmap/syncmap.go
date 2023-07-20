package syncmap

import "sync"

type Map[K comparable, V any] struct {
	m sync.Map
}

func (m *Map[K, V]) Delete(k K) { m.m.Delete(k) }

func (m *Map[K, V]) Load(k K) (value V, ok bool) {
	v, ok := m.m.Load(k)
	if v == nil {
		return value, ok
	}
	return v.(V), ok
}

func (m *Map[K, V]) LoadAndDelete(k K) (value V, loaded bool) {
	v, loaded := m.m.LoadAndDelete(k)
	if v == nil {
		return value, loaded
	}
	return v.(V), loaded
}

func (m *Map[K, V]) LoadOrStore(k K, v V) (actual V, loaded bool) {
	a, loaded := m.m.LoadOrStore(k, v)
	return a.(V), loaded
}

func (m *Map[K, V]) Range(f func(k K, v V) bool) {
	m.m.Range(func(k, v any) bool { return f(k.(K), v.(V)) })
}

func (m *Map[K, V]) Store(k K, v V) { m.m.Store(k, v) }
