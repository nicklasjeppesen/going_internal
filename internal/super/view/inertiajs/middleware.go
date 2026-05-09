package inertiajs

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
)

/*
- return the current react version to inertia
*/
func version() string {

	// Sti til din Vite manifest fil
	path := "./public/.vite/manifest.json"

	// Læs hele filen
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err.Error())
	}
	// Beregn MD5 hash af indholdet
	hash := md5.Sum(data)

	// Konverter til hex streng
	hashStr := hex.EncodeToString(hash[:])
	return hashStr
}

func InertiaMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Create context of inertia version
		inertiaVersion := version()
		ctx := context.WithValue(r.Context(), HEADER_X_INERTIA_VERSION, inertiaVersion)

		// handle share information in InertiaJs

		// Set header: vary -> Inertia
		r.Header.Set("Vary", HEADER_X_INERTIA)

		// full load, return do nothing.
		if inertia := r.Header.Get(HEADER_X_INERTIA); inertia == "" {
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {

			// Check if request is GET and Then check if inertia versions does match and handle that
			if r.Method == http.MethodGet && r.Header.Get(HEADER_X_INERTIA_VERSION) != inertiaVersion {
				r.Header.Set(HEADER_X_INERTIA_LOCATION, r.URL.String())
			}
			next.ServeHTTP(w, r.WithContext(ctx))

		}
	})
}
