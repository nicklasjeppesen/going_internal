// Package app orchestrates the core application life cycle. It handles
// initialization, loading environment variables, setting up the HTTP server
// with routing, optional TLS, static asset serving,
// and managing a graceful shutdown process.
package app

import (
	"context"
	"embed"
	"log"

	Scheduler "github.com/nicklasjeppesen/going_internal/super/jobs"

	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	middleware "github.com/nicklasjeppesen/going_internal/super/middleware"

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

// NewApp creates, configures, and returns a pointer to a new App instance.
// It initializes the router, background scheduler, and automatically loads the .env file.
func (app App) Start() {

	// 1. Setup services
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	recoveryHandler := middleware.PanicRecovery(app.Router)

	// Serve static files from the "assets" directory
	fs := http.FileServer(http.Dir("internal/resources/assets"))
	app.Router.Handle("GET /assets/", http.StripPrefix("/assets/", fs))

	// 2. Setup HTTP layer
	server := &http.Server{
		Addr:    getPort(),
		Handler: recoveryHandler,
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

func getPort() string {
	return ":" + os.Getenv("APP_PORT")
}

func GetURl() string {
	var host = os.Getenv("APP_URL")
	var url = host + getPort()
	return url
}
