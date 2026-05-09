package security

import (
	"errors"
	"myapp/internal/super/constants"
	"myapp/internal/super/util"
	"strconv"
	"time"

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
}

// Generate Token
func (s *JWTService) Generate(id int64) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(id, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
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
