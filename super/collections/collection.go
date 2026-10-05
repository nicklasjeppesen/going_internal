package collections

type Collection[T any] []T

func (c Collection[T]) Add(items ...T) Collection[T] {
	c = append(c, items...)
	return c
}

type Jsonable interface {
	ToJson() map[string]any
}

func (c Collection[T]) ToJson() []any {
	finalData := make([]any, len(c))
	for i, item := range c {
		if j, ok := any(item).(Jsonable); ok {
			customData := j.ToJson()
			finalData[i] = customData
		} else {
			finalData[i] = item
		}
	}
	return finalData
}

// Reverse returns a new collection with the elements in reverse order
// (e.g. newest-first from the database → oldest-first for display).
func (c Collection[T]) Reverse() Collection[T] {
	out := make(Collection[T], len(c))
	for i, item := range c {
		out[len(c)-1-i] = item
	}
	return out
}
