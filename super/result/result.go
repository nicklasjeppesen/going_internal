package result

type Result[T any] struct {
	Error        bool
	ErrorMessage error
	Data         T
}

func (result *Result[T]) GetErrors() string {
	if result.ErrorMessage == nil {
		return ""
	} else {
		return result.ErrorMessage.Error()
	}
}

func (c *Result[T]) And(handler func(*T) error) Result[T] {
	if c.Error {
		return *c
	} else {
		errors := handler(&c.Data)
		return Result[T]{ErrorMessage: errors, Error: errors != nil, Data: c.Data}
	}
}
