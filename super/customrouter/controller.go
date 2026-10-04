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
// gives them AddBeforeAction/AddAfterAction, typically called from Loader().
//
// Controllers registered as a pointer get a fresh instance per request (see
// buildControllerAction), so a BeforeAction may set per-request fields such
// as the current user:
//
//	type HomeController struct {
//		customrouter.BaseController
//		logger helper.ILogger // dependency, resolved from the Container
//		userID string         // per request, set by setUser
//	}
//
//	func (c *HomeController) Loader(logger helper.ILogger) *HomeController {
//		c.logger = logger
//		c.AddBeforeAction(c.setUser).Except("Index")
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

// resetHooks clears the hooks copied from the registered controller, so the
// per-request instance only gets the ones its own Loader call registers.
func (b *BaseController) resetHooks() {
	b.beforeActions = nil
	b.afterActions = nil
}

type hookResetter interface {
	resetHooks()
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

// buildControllerAction wraps a controller action in a Modifier that, per
// request, builds the controller (via Loader, if defined), runs its matching
// BeforeAction hooks, calls the action and then runs its AfterAction hooks.
//
// A controller registered as a pointer (e.g. new(HomeController)) gets a fresh
// instance for every request: a shallow copy of the registered controller,
// then Loader is called on that copy. Dependencies come from the Container
// (cached singletons), and fields set by a BeforeAction (e.g. the current
// user) belong to that one request only — concurrent requests never share
// them, and Loader's hooks are registered once per instance.
//
// A controller registered as a value (e.g. HomeController{}) is resolved once
// and shared; its value-receiver methods work on copies anyway.
//
// Everything is resolved once at registration too, so a missing dependency or
// a misspelled action still panics at startup.
func buildControllerAction(container *Container, controller interface{}, methodName string) Modifier {
	newController := controllerFactory(container, controller)
	resolveControllerMethod(newController(), methodName) // fail fast at startup

	return func(req *request.Requestbase) {
		resolved := newController()
		action := resolveControllerMethod(resolved, methodName)

		for _, before := range beforeActionsOf(resolved) {
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

		for _, after := range afterActionsOf(resolved) {
			if !actionApplies(methodName, after.only, after.except) {
				continue
			}
			after.Handler(*req)
		}
	}
}

// controllerFactory returns a func that produces a ready-to-use controller.
// For a pointer to a struct it builds a new instance on every call (a copy of
// the registered controller with its hooks cleared, then Loader); for
// anything else it resolves once and always returns that.
func controllerFactory(container *Container, controller interface{}) func() interface{} {
	value := reflect.ValueOf(controller)
	if value.Kind() != reflect.Pointer || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		resolved := ResolveDependencies(container, controller)
		return func() interface{} { return resolved }
	}

	registered := value.Elem()
	return func() interface{} {
		instance := reflect.New(registered.Type())
		instance.Elem().Set(registered)
		if hooks, ok := instance.Interface().(hookResetter); ok {
			hooks.resetHooks()
		}
		return ResolveDependencies(container, instance.Interface())
	}
}

func beforeActionsOf(controller interface{}) []*BeforeAction {
	if provider, ok := controller.(beforeActionProvider); ok {
		return provider.BeforeActions()
	}
	return nil
}

func afterActionsOf(controller interface{}) []*AfterAction {
	if provider, ok := controller.(afterActionProvider); ok {
		return provider.AfterActions()
	}
	return nil
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
