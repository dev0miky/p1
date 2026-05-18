package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID   int64
	TenantID int64
	Role     string
	JTI      string
}

type Issuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewIssuer(secret []byte, issuer string, ttl time.Duration) *Issuer {
	return &Issuer{secret: secret, issuer: issuer, ttl: ttl}
}

func (i *Issuer) Issue(c Claims) (string, error) {
	if c.UserID <= 0 {
		return "", errors.New("UserID must be > 0")
	}
	if c.TenantID <= 0 {
		return "", errors.New("TenantID must be > 0")
	}
	if c.Role == "" {
		return "", errors.New("Role must not be empty")
	}
	now := time.Now()
	jti := uuid.NewString()
	claims := jwt.MapClaims{
		"sub":       fmt.Sprintf("%d", c.UserID),
		"tenant_id": c.TenantID,
		"role":      c.Role,
		"iss":       i.issuer,
		"iat":       now.Unix(),
		"exp":       now.Add(i.ttl).Unix(),
		"jti":       jti,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(i.secret)
}

func (i *Issuer) Verify(token string) (Claims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(i.issuer))
	if err != nil {
		return Claims{}, err
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return Claims{}, errors.New("invalid claims")
	}
	c := Claims{
		Role: stringClaim(mc, "role"),
		JTI:  stringClaim(mc, "jti"),
	}
	if v, ok := mc["sub"].(string); ok {
		fmt.Sscanf(v, "%d", &c.UserID)
	}
	if v, ok := mc["tenant_id"].(float64); ok {
		c.TenantID = int64(v)
	}
	return c, nil
}

func stringClaim(m jwt.MapClaims, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}
