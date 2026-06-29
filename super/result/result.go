package result

type Result[T any] struct {
	HasError bool
	Errors   map[string][]string
	Data     T
}

func (result *Result[T]) GetErrors() map[string][]string {
	if result.HasError {
		return result.Errors
	}
	return nil
}

func (c *Result[T]) And(handler func(*T) error) Result[T] {
	if c.HasError {
		return *c
	} else {
		if errors := handler(&c.Data); errors != nil {
			c.Errors["errors"] = append(c.Errors["errors"], errors.Error())
			c.HasError = true
		}
		return *c
	}
}
