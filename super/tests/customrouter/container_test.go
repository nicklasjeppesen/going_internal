package customrouter

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/customrouter"
)

// Tests for the DI container: factories with parameters (autowiring),
// Resolve, cycles, missing types and concurrent resolving.

// store, repository and notifier are what the factories build.
type store struct{ dsn string }

type repository interface{ Store() *store }

type todoRepository struct{ store *store }

func (r todoRepository) Store() *store { return r.store }

type notifier interface{ Repo() repository }

type pushNotifier struct{ repo repository }

func (n pushNotifier) Repo() repository { return n.repo }

// cycleA and cycleB need each other.
type cycleA interface{ A() }
type cycleB interface{ B() }
type cycleC interface{ C() }

// wantPanic fails the test unless call panics with a message containing every
// one of want.
func wantPanic(t *testing.T, call func(), want ...string) {
	t.Helper()
	defer func() {
		t.Helper()
		recovered := recover()
		if recovered == nil {
			t.Fatalf("expected a panic containing %q", want)
		}
		message := fmt.Sprint(recovered)
		for _, w := range want {
			if !strings.Contains(message, w) {
				t.Fatalf("panic %q, want it to contain %q", message, w)
			}
		}
	}()
	call()
}

func TestFactoriesGetTheirParametersFromTheContainer(t *testing.T) {
	var built atomic.Int32
	container := customrouter.NewContainer().
		Register(func(repo repository) notifier { return pushNotifier{repo} }). // registered before what it needs
		Register(func(s *store) repository { built.Add(1); return todoRepository{s} }).
		Register(func() *store { return &store{dsn: "kontoret.db"} })

	n := customrouter.Resolve[notifier](container)
	if n.Repo().Store().dsn != "kontoret.db" {
		t.Fatalf("notifier: %+v", n)
	}
	// Singletons: the repository the notifier got is the one Resolve gives.
	if repo := customrouter.Resolve[repository](container); repo != n.Repo() || built.Load() != 1 {
		t.Fatalf("the repository must be built once, built %d times", built.Load())
	}
	if customrouter.Resolve[*store](container) != n.Repo().Store() {
		t.Fatal("the store must be a singleton")
	}

	value, ok := container.Resolve(reflect.TypeFor[repository]())
	if !ok || value.Interface() != n.Repo() {
		t.Fatalf("Resolve: %v %v", value, ok)
	}
	if _, ok := container.Resolve(reflect.TypeFor[cycleC]()); ok {
		t.Fatal("Resolve of an unregistered type must be false")
	}
}

func TestResolveOfAnUnregisteredTypePanics(t *testing.T) {
	container := customrouter.NewContainer()
	wantPanic(t, func() { customrouter.Resolve[repository](container) }, "nothing is registered", "repository")
}

func TestAMissingParameterPanicsWithTheFactoryAndTheType(t *testing.T) {
	container := customrouter.NewContainer().
		Register(func(repo repository) notifier { return pushNotifier{repo} }).
		Register(func(s *store) repository { return todoRepository{s} })

	wantPanic(t, func() { customrouter.Resolve[notifier](container) },
		"the factory for customrouter.repository needs *customrouter.store", "not registered",
		"customrouter.notifier → customrouter.repository → *customrouter.store")
}

func TestACyclePanicsWithTheChain(t *testing.T) {
	container := customrouter.NewContainer().
		Register(func(cycleB) cycleA { return nil }).
		Register(func(cycleC) cycleB { return nil }).
		Register(func(cycleA) cycleC { return nil })

	wantPanic(t, func() { customrouter.Resolve[cycleA](container) },
		"dependency cycle: customrouter.cycleA → customrouter.cycleB → customrouter.cycleC → customrouter.cycleA")
}

func TestResolveIsSafeForConcurrentUse(t *testing.T) {
	var built atomic.Int32
	container := customrouter.NewContainer().
		Register(func() *store { built.Add(1); return &store{} }).
		Register(func(s *store) repository { built.Add(1); return todoRepository{s} }).
		Register(func(repo repository) notifier { built.Add(1); return pushNotifier{repo} })

	var wg sync.WaitGroup
	results := make([]notifier, 50)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				results[i] = customrouter.Resolve[notifier](container)
			} else {
				customrouter.Resolve[repository](container)
				results[i] = customrouter.Resolve[notifier](container)
			}
		}()
	}
	wg.Wait()
	for _, n := range results {
		if n != results[0] {
			t.Fatal("every goroutine must get the same notifier")
		}
	}
	if built.Load() != 3 {
		t.Fatalf("each factory must run once, %d calls", built.Load())
	}
}

func TestZeroArgumentFactoriesStillWork(t *testing.T) {
	container := customrouter.NewContainer().Register(func() repository { return todoRepository{&store{dsn: "x"}} })
	if customrouter.Resolve[repository](container).Store().dsn != "x" {
		t.Fatal("zero-argument factory")
	}
	wantPanic(t, func() { container.Register(func() (repository, error) { return nil, nil }) }, "exactly one value")
	wantPanic(t, func() { container.Register(42) }, "expects a func")
}

// notifierController gets an autowired dependency through Loader.
type notifierController struct{ notifier notifier }

func (c *notifierController) Loader(n notifier) *notifierController {
	c.notifier = n
	return c
}

func TestLoaderGetsAutowiredDependencies(t *testing.T) {
	container := customrouter.NewContainer().
		Register(func() *store { return &store{dsn: "loader"} }).
		Register(func(s *store) repository { return todoRepository{s} }).
		Register(func(repo repository) notifier { return pushNotifier{repo} })

	c := customrouter.ResolveDependencies(container, &notifierController{}).(*notifierController)
	if c.notifier.Repo().Store().dsn != "loader" {
		t.Fatalf("Loader: %+v", c.notifier)
	}
}
