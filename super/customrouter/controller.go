package customrouter

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/nicklasjeppesen/going_internal/super/request"
)

// ---- Controller lifecycle hooks (Rails-style before/after actions) ----

// BeforeAction describes a single "before" hook a controller wants to run
// ahead of one or more of its actions.
//
//   - Only, if non-empty, restricts the hook to the listed action (method)
//   - Except, if non-empty (and Only is empty), runs the hook for every
//     action *except* the listed ones.
//   - If both Only and Except are empty, the hook runs for every action.
//   - Handler returns false to abort the request. It's responsible for
//     writing a response itself in that case (e.g. 401/redirect).
//
// Action names are matched case-insensitively against the method name used
// when registering the route (e.g. Get("/x", c, "Show") -> action "Show").
type BeforeAction struct {
	only    []string
	except  []string
	Handler func(request request.Requestbase) bool
}

func (b *BeforeAction) Only(actions ...string) *BeforeAction {
	b.only = actions
	return b
}

func (b *BeforeAction) Except(actions ...string) *BeforeAction {
	b.except = actions
	return b
}

// AfterAction is the "after" counterpart to BeforeAction. It always runs
// once the main action has completed (unless a BeforeAction aborted the
// request first).
type AfterAction struct {
	only    []string
	except  []string
	Handler func(request request.Requestbase)
}

func (b *AfterAction) Only(actions ...string) *AfterAction {
	b.only = actions
	return b
}

func (b *AfterAction) Except(actions ...string) *AfterAction {
	b.except = actions
	return b
}

// BaseController is meant to be embedded (by value) in your controllers. It
// gives them AddBeforeAction/AddAfterAction, typically called from Loader(),
// e.g.:
//
//	type HomeController struct {
//		customrouter.BaseController
//	}
//
//	func (c *HomeController) Loader() *HomeController {
//		c.AddBeforeAction(customrouter.BeforeAction{
//			Name:    "authenticate_user",
//			Handler: c.authenticateUser,
//		})
//		return c
//	}
type BaseController struct {
	beforeActions []*BeforeAction
	afterActions  []*AfterAction
}

// AddBeforeAction registers a before-hook on the controller.
func (b *BaseController) AddBeforeAction(action func(request request.Requestbase) bool) *BeforeAction {
	var before = &BeforeAction{
		Handler: action,
	}
	b.beforeActions = append(b.beforeActions, before)
	return before
}

// AddAfterAction registers an after-hook on the controllser.
func (b *BaseController) AddAfterAction(action func(request request.Requestbase)) *AfterAction {
	var after = &AfterAction{
		Handler: action,
	}
	b.afterActions = append(b.afterActions, after)
	return after
}

// BeforeActions returns the hooks registered via AddBeforeAction, in order.
func (b *BaseController) BeforeActions() []*BeforeAction {
	return b.beforeActions
}

// AfterActions returns the hooks registered via AddAfterAction, in order.
func (b *BaseController) AfterActions() []*AfterAction { return b.afterActions }

// beforeActionProvider / afterActionProvider are satisfied by BaseController
// (via the embedded methods above). The router type-asserts against these
// rather than against BaseController directly, so a controller isn't forced
// to embed exactly that struct if it wants to provide the hooks another way.
type beforeActionProvider interface {
	BeforeActions() []*BeforeAction
}

type afterActionProvider interface {
	AfterActions() []*AfterAction
}

// actionApplies implements the Only/Except matching rules described on BeforeAction.
func actionApplies(actionName string, only, except []string) bool {
	if len(only) > 0 {
		return containsFold(only, actionName)
	}
	if len(except) > 0 {
		return !containsFold(except, actionName)
	}
	return true
}

func containsFold(list []string, s string) bool {
	for _, item := range list {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}

// buildControllerAction resolves a controller's dependencies (via Loader, if
// defined), looks up the target action method, and wraps everything in a
// Modifier that also runs any matching BeforeAction/AfterAction hooks the
// resolved controller has registered.
//
// Resolution (including the Loader call) happens once, at route-registration
// time — not per request — so it behaves like constructor injection. If you
// need per-request/scoped dependencies (e.g. the current user), fetch them in
// a BeforeAction instead, using the request/context.
func buildControllerAction(container *Container, controller interface{}, methodName string) Modifier {
	resolved := ResolveDependencies(container, controller)
	action := resolveControllerMethod(resolved, methodName)

	var befores []*BeforeAction
	if provider, ok := resolved.(beforeActionProvider); ok {
		befores = provider.BeforeActions()
	}

	var afters []*AfterAction
	if provider, ok := resolved.(afterActionProvider); ok {
		afters = provider.AfterActions()
	}

	return func(req *request.Requestbase) {

		//_request := request.Requestbase{W: w, R: r}

		for _, before := range befores {
			if !actionApplies(methodName, before.only, before.except) {
				continue
			}

			if !before.Handler(*req) {
				return // aborted — handler is responsible for the response
			}
		}

		var urlParamKeys = extractPathParams(req.R.Pattern)
		var urlParam []string
		for _, key := range urlParamKeys {
			urlParam = append(urlParam, req.R.PathValue(key))
		}
		request.CallUnknownFunc(action, urlParam, req.W, req.R)

		for _, after := range afters {
			if !actionApplies(methodName, after.only, after.except) {
				continue
			}
			after.Handler(*req)
		}
	}
}

// resolveController checks whether the controller defines a Loader(...) method.
// If it does, each of Loader's parameters is treated as an interface type and
// resolved from the container, then Loader is called.
//
// Loader can either:
//   - return the initialized controller (constructor style) — that return
//     value becomes the "real" controller instance, or
//   - return nothing, and instead mutate the controller in place (e.g. via
//     AddBeforeAction) — in which case the original controller (on which
//     Loader was called) is used as-is.
//
// If there's no Loader method at all, the controller is used as-is.
func ResolveDependencies(container *Container, controller interface{}) interface{} {
	value := reflect.ValueOf(controller)
	loader := value.MethodByName("Loader")
	if !loader.IsValid() {
		return controller
	}

	loaderType := loader.Type()
	args := make([]reflect.Value, loaderType.NumIn())
	for i := 0; i < loaderType.NumIn(); i++ {
		paramType := loaderType.In(i)
		if container == nil {
			panic(fmt.Sprintf("customrouter: %T defines Loader(...) with parameters but no Container was configured — call router.UseContainer(...)", controller))
		}
		dep, ok := container.resolve(paramType)
		if !ok {
			panic(fmt.Sprintf("customrouter: no dependency registered for %s (required by %T.Loader, param %d)", paramType, controller, i))
		}
		args[i] = dep
	}

	results := loader.Call(args)
	if len(results) > 0 {
		return results[0].Interface()
	}
	return controller
}

// resolveControllerMethod looks up a method by name on a controller instance using
// reflection. Panics (at route-registration time, i.e. at startup) if the method
// doesn't exist, so misconfigured routes fail fast instead of at request time.
//
// Note: if Home has a pointer receiver (func (c *HomeController) Home(...)),
// you must pass a pointer to the controller (e.g. &HomeController{}), not a value.
func resolveControllerMethod(controller interface{}, methodName string) interface{} {
	value := reflect.ValueOf(controller)
	method := value.MethodByName(methodName)
	if !method.IsValid() {
		panic(fmt.Sprintf("customrouter: method %q not found on controller %T (check spelling and pointer receiver)", methodName, controller))
	}
	return method.Interface()
}
