// Package app orchestrates the core application life cycle. It handles
// initialization, loading environment variables, setting up the HTTP server
// with routing, optional TLS, static asset serving,
// and managing a graceful shutdown process.
package app

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/go-playground/validator/v10"
	Scheduler "github.com/nicklasjeppesen/going_internal/super/jobs"
	"github.com/nicklasjeppesen/going_internal/super/validation"

	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	middleware "github.com/nicklasjeppesen/going_internal/super/middleware"
	"github.com/nicklasjeppesen/going_internal/super/request"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// App aggregates the essential components of the application, including the
// HTTP router, the background job scheduler, static assets, and configurations
// for TLS and view engines.
type App struct {
	// Router is the primary multiplexer for handling incoming HTTP requests.
	Router *http.ServeMux

	// Scheduler manages background cron jobs and tasks.
	Scheduler *Scheduler.Scheduler

	// EmbeddedFiles holds static or template assets embedded into the binary.
	EmbeddedFiles embed.FS

	// UseTLS indicates whether the HTTP server should start with TLS (HTTPS) enabled.
	UseTLS bool
}

func NewApp() *App {
	app := new(App)
	app.Router = http.NewServeMux()
	app.Scheduler = Scheduler.New()
	app.LoadEnv()
	app.UseTLS = true // Set to true to enable TLS, false for HTTP
	return app
}

func (app App) RegisterCustomRules(rules map[string]func(fl validator.FieldLevel) bool) {
	for tag, fn := range rules {
		if err := validation.RegisterValidation(tag, fn); err != nil {
			fmt.Errorf("failed to register validation rule %q: %w", tag, err)
			panic(err)
		}
	}
}

// NewApp creates, configures, and returns a pointer to a new App instance.
// It initializes the router, background scheduler, and automatically loads the .env file.
func (app App) Start() {

	// 1. Setup services
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	recoveryHandler := middleware.PanicRecovery(middleware.SecurityHeaders(limitRequestBody(app.Router, maxBodyBytes())))

	// Serve static files from the "assets" directory
	fs := http.FileServer(http.Dir("internal/resources/assets"))
	app.Router.Handle("GET /assets/", http.StripPrefix("/assets/", withServiceWorkerAllowed(fs)))

	// 2. Setup HTTP layer
	server := &http.Server{
		Addr:    getPort(),
		Handler: recoveryHandler,
		// Without timeouts a client can hold a connection (and a goroutine)
		// open forever by sending its request very slowly ("slowloris").
		// Websockets are not affected: the upgrade clears these deadlines.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 3. Start server
	go func() {

		log.Printf("Server running at %s:%s", os.Getenv("APP_URL"), os.Getenv("APP_PORT"))
		var err error
		if app.UseTLS {
			// Ensure cert.pem and key.pem exist in your root or provide paths via ENV
			err = server.ListenAndServeTLS("cert.pem", "key.pem")
		} else {
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 4. Graceful Shutdown
	<-ctx.Done()
	handleShutDown(server, app.Scheduler)
}

// handleShutDown stops the HTTP server and background scheduler gracefully,
// allowing active requests up to 10 seconds to finish.
func handleShutDown(server *http.Server, s *Scheduler.Scheduler) {
	// Stop the webserver in a nice way
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)

	s.Stop()
	log.Println("shutdown finish")

}

func (app App) LoadEnv() {
	// Load .env file
	enverr := godotenv.Load()
	if enverr != nil {
		log.Fatalf("Error loading .env file")
	}
}

// defaultMaxBodyBytes caps a request body (10 MB) unless APP_MAX_BODY_BYTES
// says otherwise. Bodies are read into memory (JSON, CSRF check), so without a
// cap one request could make the server allocate gigabytes.
const defaultMaxBodyBytes = 10 << 20

func maxBodyBytes() int64 {
	if n, err := strconv.ParseInt(os.Getenv("APP_MAX_BODY_BYTES"), 10, 64); err == nil && n > 0 {
		return n
	}
	return defaultMaxBodyBytes
}

// limitRequestBody makes reading more than limit bytes of a request body fail,
// before any handler or middleware reads it. A route can raise the limit for
// itself, e.g. for uploads: Post(...).MaxBody(25 << 20).
func limitRequestBody(next http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, request.LimitBody(w, r, limit))
	})
}

func getPort() string {
	return ":" + os.Getenv("APP_PORT")
}

// withServiceWorkerAllowed sets the Service-Worker-Allowed header to "/" so
// service workers served from the assets tree can be registered with the root
// scope, controlling the whole origin, instead of being limited to the scope
// of the directory they live in.
//
// The header is only honoured by the browser when the response is requested as
// a service worker script, so it is harmless for any other static asset.
func withServiceWorkerAllowed(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Service-Worker-Allowed", "/")
		next.ServeHTTP(w, r)
	})
}

func GetURl() string {
	var host = os.Getenv("APP_URL")
	var url = host + getPort()
	return url
}
