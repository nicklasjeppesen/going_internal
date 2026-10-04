package security

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"time"

	"github.com/nicklasjeppesen/going_internal/super/constants"

	"golang.org/x/crypto/bcrypt"
)

func GenerateToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatal("Failed to generate token v%", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

func HashPassword(password string) string {
	bytes, _ := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(bytes)

}

func CheckPasswordhash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// dummyHash is compared against when a login names an unknown user, so the
// response takes as long as for a wrong password (no way to tell which emails
// exist from the timing).
var dummyHash = HashPassword("going-timing-equalizer")

// CheckPasswordAgainstNothing spends the same time as CheckPasswordhash.
func CheckPasswordAgainstNothing(password string) {
	CheckPasswordhash(password, dummyHash)
}

// LoginUser sets the login cookie: a JWT for the user, bound to sessionID.
// SameSite=Lax: the cookie is not sent with cross-site POST/fetch requests
// (extra protection against CSRF), but still with normal links to the app.
func LoginUser(id int64, sessionID string, w http.ResponseWriter) string {
	svc := NewJWTService()
	token, _ := svc.GenerateForSession(id, sessionID)
	http.SetCookie(w, &http.Cookie{
		Name:     constants.Auth_token,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(tokenLifetime),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})
	return token
}

func Logout(w http.ResponseWriter) {

	// clear cookie:
	http.SetCookie(w, &http.Cookie{
		Name:     constants.Auth_token,
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-time.Hour),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     constants.Csrf_token,
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	})

}
