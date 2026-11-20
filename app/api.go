package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

func apiRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v2/auth/token", apiToken)
	mux.HandleFunc("POST /api/v2/register", apiRegister)
	mux.HandleFunc("GET /api/v2/me", needLogin(apiMe))
	mux.HandleFunc("GET /api/v2/shipments", needLogin(apiShipments))
	mux.HandleFunc("GET /api/v2/shipments/count", needLogin(apiShipmentCount))
	mux.HandleFunc("GET /api/v2/shipments/{id}", needLogin(apiShipment))
	mux.HandleFunc("POST /api/v2/webhooks/test", needLogin(apiWebhookTest))
	mux.HandleFunc("GET /api/v2/admin/users", needAdmin(apiAdminUsers))
}

func apiToken(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in) != nil {
		jsonOut(w, 400, map[string]any{"error": "invalid json"})
		return
	}
	var id int
	var role, hash string
	err := db.QueryRow("SELECT id, role, password_hash FROM users WHERE username=?", in.Username).Scan(&id, &role, &hash)
	if err != nil || hash != hashPW(in.Password) {
		appLog.Printf("auth token user=%s password=%s result=fail", in.Username, in.Password)
		jsonOut(w, 401, map[string]any{"error": "invalid credentials"})
		return
	}
	now := time.Now()
	tok := signJWT(map[string]any{"uid": id, "sub": in.Username, "role": role, "iat": now.Unix(), "exp": now.Add(8 * time.Hour).Unix()})
	appLog.Printf("auth token user=%s result=ok jwt=%s", in.Username, tok)
	jsonOut(w, 200, map[string]any{"token": tok, "token_type": "Bearer", "expires_in": 28800})
}

func apiRegister(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username    string `json:"username"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		FullName    string `json:"full_name"`
		Company     string `json:"company"`
		AccountType string `json:"account_type"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in) != nil {
		jsonOut(w, 400, map[string]any{"error": "invalid json"})
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if len(in.Username) < 3 || len(in.Password) < 6 || !strings.Contains(in.Email, "@") || strings.TrimSpace(in.Company) == "" {
		jsonOut(w, 400, map[string]any{"error": "username (3+), password (6+), e-mail and company are required"})
		return
	}
	var n int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE username=?", in.Username).Scan(&n)
	if n > 0 {
		jsonOut(w, 409, map[string]any{"error": "username already taken"})
		return
	}
	role := in.AccountType
	if role == "" {
		role = "customer"
	}
	res, err := db.Exec("INSERT INTO companies (name, industry, billing_email, country) VALUES (?, 'Unspecified', ?, 'N/A')", in.Company, in.Email)
	if err != nil {
		jsonOut(w, 500, map[string]any{"error": "could not create company"})
		return
	}
	cid, _ := res.LastInsertId()
	res, err = db.Exec("INSERT INTO users (company_id, username, email, full_name, password_hash, role) VALUES (?,?,?,?,?,?)",
		cid, in.Username, in.Email, in.FullName, hashPW(in.Password), role)
	if err != nil {
		jsonOut(w, 500, map[string]any{"error": "could not create user"})
		return
	}
	uid, _ := res.LastInsertId()
	appLog.Printf("auth register user=%s password=%s role=%s", in.Username, in.Password, role)
	jsonOut(w, 201, map[string]any{"status": "created", "id": uid, "username": in.Username, "role": role})
}

func apiMe(w http.ResponseWriter, r *http.Request, a *Auth) {
	var email, phone, full, cname string
	db.QueryRow("SELECT u.email, COALESCE(u.phone,''), u.full_name, c.name FROM users u JOIN companies c ON c.id=u.company_id WHERE u.id=?", a.UID).Scan(&email, &phone, &full, &cname)
	jsonOut(w, 200, map[string]any{"id": a.UID, "username": a.Username, "role": a.Role, "company_id": a.CompanyID,
		"company": cname, "email": email, "phone": phone, "full_name": full})
}

func apiShipments(w http.ResponseWriter, r *http.Request, a *Auth) {
	out := []map[string]any{}
	rows, err := db.Query("SELECT "+shipCols+" FROM shipments s JOIN companies c ON c.id=s.company_id WHERE s.company_id=? ORDER BY s.id DESC", a.CompanyID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			if s, e := scanShip(rows); e == nil {
				out = append(out, map[string]any{"id": s.ID, "ref": s.Ref, "mode": s.Mode, "origin": s.Origin, "destination": s.Dest,
					"status": s.Status, "eta": s.ETA, "weight_kg": s.WeightKG})
			}
		}
	}
	jsonOut(w, 200, map[string]any{"count": len(out), "shipments": out})
}

// returns the number of this company's shipments in a given status
func apiShipmentCount(w http.ResponseWriter, r *http.Request, a *Auth) {
	status := r.URL.Query().Get("status")
	q := fmt.Sprintf("SELECT COUNT(*) FROM shipments WHERE company_id = %d AND status = '%s'", a.CompanyID, status)
	var n int
	if err := db.QueryRow(q).Scan(&n); err != nil {
		appLog.Printf("api count query failed: %v", err)
		jsonOut(w, 400, map[string]any{"error": "query failed"})
		return
	}
	jsonOut(w, 200, map[string]any{"status": status, "count": n})
}

func apiShipment(w http.ResponseWriter, r *http.Request, a *Auth) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		// debugging aid left enabled
		jsonOut(w, 500, map[string]any{
			"error": err.Error(),
			"where": "api.apiShipment",
			"db":    dbDSNSafe,
			"trace": string(debug.Stack()),
		})
		return
	}
	s, err := scanShip(db.QueryRow("SELECT "+shipCols+" FROM shipments s JOIN companies c ON c.id=s.company_id WHERE s.id=? AND s.company_id=?", id, a.CompanyID))
	if err != nil {
		jsonOut(w, 404, map[string]any{"error": "shipment not found"})
		return
	}
	ev := []map[string]string{}
	for _, e := range eventsFor(s.ID) {
		ev = append(ev, map[string]string{"time": e.When, "location": e.Where, "description": e.What})
	}
	jsonOut(w, 200, map[string]any{"id": s.ID, "ref": s.Ref, "mode": s.Mode, "origin": s.Origin, "destination": s.Dest,
		"status": s.Status, "container": s.Container, "weight_kg": s.WeightKG, "eta": s.ETA, "events": ev})
}

func apiWebhookTest(w http.ResponseWriter, r *http.Request, a *Auth) {
	var in struct {
		URL string `json:"url"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in) != nil || in.URL == "" {
		jsonOut(w, 400, map[string]any{"error": "url required"})
		return
	}
	cl := &http.Client{Timeout: 6 * time.Second}
	resp, err := cl.Get(in.URL)
	if err != nil {
		jsonOut(w, 200, map[string]any{"reachable": false, "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	jsonOut(w, 200, map[string]any{"reachable": true, "status": resp.Status, "body": string(b)})
}

func apiAdminUsers(w http.ResponseWriter, r *http.Request, a *Auth) {
	out := []map[string]any{}
	rows, err := db.Query("SELECT id, username, email, role, password_hash FROM users ORDER BY id")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int
			var u, e, ro, h string
			rows.Scan(&id, &u, &e, &ro, &h)
			out = append(out, map[string]any{"id": id, "username": u, "email": e, "role": ro, "password_hash": h})
		}
	}
	jsonOut(w, 200, map[string]any{"users": out})
}
