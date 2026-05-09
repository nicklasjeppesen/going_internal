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

func LoginUser(id int64, w http.ResponseWriter) string {
	svc := NewJWTService()
	token, _ := svc.Generate(id)
	http.SetCookie(w, &http.Cookie{
		Name:     constants.Auth_token,
		Value:    token,
		Expires:  time.Now().Add(42 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	})
	return token
}

func Logout(w http.ResponseWriter) {

	// clear cookie:
	http.SetCookie(w, &http.Cookie{
		Name:     constants.Auth_token,
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HttpOnly: true,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     constants.Csrf_token,
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HttpOnly: false,
	})

}
