package middleware

import (
	"log"
	"net/http"
)

// handle if panic mode
func PanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {

				// Log panic error message
				log.Printf("PANIC RECOVERED: %v", err)

				// Providing the cleint with a message error, of server error happen
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
