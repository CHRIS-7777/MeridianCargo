package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Auth struct {
	UID       int
	Username  string
	Role      string
	CompanyID int
}

func (a *Auth) IsStaff() bool { return a != nil && (a.Role == "staff" || a.Role == "admin") }
func (a *Auth) IsAdmin() bool { return a != nil && a.Role == "admin" }

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func randPIN() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(10000))
	s := n.String()
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

// passwords are stored as plain SHA-1 digests
func hashPW(pw string) string {
	h := sha1.Sum([]byte(pw))
	return hex.EncodeToString(h[:])
}

// ---- JWT (HS256) -----------------------------------------------------------

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
func unb64(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

func signJWT(claims map[string]any) string {
	hdr := b64([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pl, _ := json.Marshal(claims)
	unsigned := hdr + "." + b64(pl)
	m := hmac.New(sha256.New, jwtSecret)
	m.Write([]byte(unsigned))
	return unsigned + "." + b64(m.Sum(nil))
}

func parseJWT(tok string) (map[string]any, bool) {
	parts := strings.Split(tok, ".")
	if len(parts) < 2 {
		return nil, false
	}
	hb, err := unb64(parts[0])
	if err != nil {
		return nil, false
	}
	var hdr map[string]any
	if json.Unmarshal(hb, &hdr) != nil {
		return nil, false
	}
	alg, _ := hdr["alg"].(string)
	pb, err := unb64(parts[1])
	if err != nil {
		return nil, false
	}
	var claims map[string]any
	if json.Unmarshal(pb, &claims) != nil {
		return nil, false
	}
	if !strings.EqualFold(alg, "none") {
		if len(parts) != 3 {
			return nil, false
		}
		m := hmac.New(sha256.New, jwtSecret)
		m.Write([]byte(parts[0] + "." + parts[1]))
		sig, err := unb64(parts[2])
		if err != nil || !hmac.Equal(sig, m.Sum(nil)) {
			return nil, false
		}
	}
	if exp, ok := claims["exp"].(float64); ok && time.Now().Unix() > int64(exp) {
		return nil, false
	}
	return claims, true
}

// ---- current user ----------------------------------------------------------

func authUser(r *http.Request) *Auth {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		if c, ok := parseJWT(strings.TrimPrefix(h, "Bearer ")); ok {
			uidF, _ := c["uid"].(float64)
			sub, _ := c["sub"].(string)
			role, _ := c["role"].(string)
			a := &Auth{UID: int(uidF), Username: sub, Role: role}
			db.QueryRow("SELECT company_id FROM users WHERE id=?", a.UID).Scan(&a.CompanyID)
			return a
		}
		return nil
	}
	s := sessionOf(r)
	if s.UID > 0 {
		return &Auth{UID: s.UID, Username: s.Username, Role: s.Role, CompanyID: s.CompanyID}
	}
	return nil
}

func isAPI(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/api/") }

func needLogin(h func(http.ResponseWriter, *http.Request, *Auth)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := authUser(r)
		if a == nil {
			if isAPI(r) {
				jsonOut(w, 401, map[string]any{"error": "authentication required"})
				return
			}
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
			return
		}
		h(w, r, a)
	}
}

func needStaff(h func(http.ResponseWriter, *http.Request, *Auth)) http.HandlerFunc {
	return needLogin(func(w http.ResponseWriter, r *http.Request, a *Auth) {
		if !a.IsStaff() {
			if isAPI(r) {
				jsonOut(w, 403, map[string]any{"error": "forbidden"})
				return
			}
			renderStatus(w, r, 403, "Access denied", "Your account does not have access to the operations console.")
			return
		}
		h(w, r, a)
	})
}

func needAdmin(h func(http.ResponseWriter, *http.Request, *Auth)) http.HandlerFunc {
	return needLogin(func(w http.ResponseWriter, r *http.Request, a *Auth) {
		if !a.IsAdmin() {
			if isAPI(r) {
				jsonOut(w, 403, map[string]any{"error": "forbidden"})
				return
			}
			renderStatus(w, r, 403, "Access denied", "Administrator privileges are required.")
			return
		}
		h(w, r, a)
	})
}
