package security

import (
	"errors"
	"strconv"
	"time"

	"github.com/nicklasjeppesen/going_internal/super/constants"
	"github.com/nicklasjeppesen/going_internal/super/util"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secret []byte
}

func NewJWTService() *JWTService {
	secret := util.GetEnv(constants.APP_Key, "")
	return &JWTService{secret: []byte(secret)}
}

type Claims struct {
	jwt.RegisteredClaims
	// SessionID ties the token to the user's current session (users.sessiontoken).
	// Logout clears it, which makes every token issued before invalid.
	SessionID string `json:"sid,omitempty"`
}

// tokenLifetime is how long a login token (and its cookie) is valid.
const tokenLifetime = 24 * time.Hour

// Generate Token
func (s *JWTService) Generate(id int64) (string, error) {
	return s.GenerateForSession(id, "")
}

// GenerateForSession creates a token for the user bound to a session id.
func (s *JWTService) GenerateForSession(id int64, sessionID string) (string, error) {
	now := time.Now()
	claims := Claims{
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(id, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenLifetime)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// Verify Token
func (s *JWTService) Verify(tokenStr string) (*jwt.Token, *Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(
		tokenStr,
		claims,
		func(t *jwt.Token) (interface{}, error) {

			// Perform validation
			if t.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return s.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	)
	if err != nil {
		return nil, nil, err
	}
	if !token.Valid {
		return nil, nil, errors.New("invalid token")
	}
	return token, claims, nil
}
