package app

import (
	"context"
	"embed"
	"log"

	Scheduler "github.com/nicklasjeppesen/going_internal/internal/super/jobs"
	"github.com/nicklasjeppesen/going_internal/internal/super/view/inertiajs"

	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	middleware "github.com/nicklasjeppesen/going_internal/internal/super/middleware"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq" // PostgreSQL driver
)

type App struct {
	Router        *http.ServeMux
	Scheduler     *Scheduler.Scheduler
	EmbeddedFiles embed.FS
}

func NewApp(embeddedFiles embed.FS) *App {
	app := new(App)
	app.Router = http.NewServeMux()
	app.Scheduler = Scheduler.New()
	app.EmbeddedFiles = embeddedFiles
	app.LoadEnv()
	return app
}

func (app App) Start() {

	// 1. Setup services
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	recoveryHandler := middleware.PanicRecovery(app.Router)

	// 2. Setup HTTP layer
	app.RegisterRoutes(app.Router, app.EmbeddedFiles)
	server := &http.Server{
		Addr:    getPort(),
		Handler: recoveryHandler,
	}

	// 3. Start server
	go func() {
		log.Printf("Server running at %s:%s", os.Getenv("APP_URL"), os.Getenv("APP_PORT"))
		if err := server.ListenAndServeTLS("cert.pem", "key.pem"); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 4. Graceful Shutdown
	<-ctx.Done()
	handleShutDown(server, app.Scheduler)
}

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

func (app App) RegisterRoutes(router *http.ServeMux, embeddedFiles embed.FS) {

	inertiajs.ViewRoter(router, embeddedFiles)

}
