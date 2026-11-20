package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"expvar"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

//go:embed templates/*.html
var tplFS embed.FS

var (
	db        *sql.DB
	pages     = map[string]*template.Template{}
	jwtSecret []byte
	appLog    *log.Logger
	dbDSNSafe string
)

type Page map[string]any

type ctxKey string

const sessKey ctxKey = "sess"

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	// ---- logging -------------------------------------------------------
	_ = os.MkdirAll("/app/logs", 0o755)
	lf, err := os.OpenFile("/app/logs/app.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	appLog = log.New(io.MultiWriter(os.Stdout, lf), "", log.LstdFlags)

	jwtSecret = []byte(env("MC_JWT_SECRET", "change-me"))

	// ---- database ------------------------------------------------------
	dbHost := env("DB_HOST", "db")
	dbUser := env("DB_USER", "mc_app")
	dbPass := env("DB_PASS", "")
	dbName := env("DB_NAME", "meridian")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true&charset=utf8mb4", dbUser, dbPass, dbHost, dbName)
	dbDSNSafe = fmt.Sprintf("%s@tcp(%s:3306)/%s", dbUser, dbHost, dbName)
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(5 * time.Minute)
	for i := 0; i < 60; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		appLog.Printf("waiting for database: %v", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}
	appLog.Printf("startup db connected dsn=%s", dbDSNSafe)

	// ---- templates -----------------------------------------------------
	loadTemplates()

	// ---- expvar diagnostics --------------------------------------------
	expvar.Publish("service", expvar.Func(func() any {
		return map[string]any{
			"name":    "meridian-portal",
			"version": "2024.11.3",
			"env":     os.Environ(),
		}
	}))

	// ---- internal-only service (loopback) -------------------------------
	go internalService()

	// ---- routes --------------------------------------------------------
	mux := http.NewServeMux()
	routes(mux)

	mux.Handle("/debug/vars", expvar.Handler())
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           middleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	appLog.Printf("listening on :8080")
	log.Fatal(srv.ListenAndServe())
}

func loadTemplates() {
	fm := template.FuncMap{
		"raw":   func(s string) template.HTML { return template.HTML(s) },
		"money": func(c int) string { return fmt.Sprintf("%d.%02d", c/100, c%100) },
	}
	entries, _ := tplFS.ReadDir("templates")
	for _, e := range entries {
		n := e.Name()
		if n == "layout.html" {
			continue
		}
		t := template.Must(template.New(n).Funcs(fm).ParseFS(tplFS, "templates/layout.html", "templates/"+n))
		pages[strings.TrimSuffix(n, ".html")] = t
	}
}

// ---------------------------------------------------------------------------
// middleware: session attach, request logging, CORS for /api/v2, recover
// ---------------------------------------------------------------------------

type statusRec struct {
	http.ResponseWriter
	code int
}

func (s *statusRec) WriteHeader(c int) { s.code = c; s.ResponseWriter.WriteHeader(c) }

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRec{ResponseWriter: w, code: 200}

		defer func() {
			if e := recover(); e != nil {
				appLog.Printf("panic: %v path=%s", e, r.URL.Path)
				http.Error(rec, "Internal Server Error", 500)
			}
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			appLog.Printf("req %s %s?%s status=%d ip=%s dur=%s", r.Method, r.URL.Path, r.URL.RawQuery, rec.code, ip, time.Since(start).Round(time.Millisecond))
		}()

		// CORS for the API surface
		if strings.HasPrefix(r.URL.Path, "/api/v2/") {
			if o := r.Header.Get("Origin"); o != "" {
				h := rec.Header()
				h.Set("Access-Control-Allow-Origin", o)
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			}
			if r.Method == http.MethodOptions {
				rec.WriteHeader(204)
				return
			}
		}

		p := r.URL.Path
		if !(strings.HasPrefix(p, "/static/") || strings.HasPrefix(p, "/exports/") || strings.HasPrefix(p, "/uploads/") || strings.HasPrefix(p, "/debug/")) {
			s := loadSession(rec, r)
			r = r.WithContext(context.WithValue(r.Context(), sessKey, s))
		}
		next.ServeHTTP(rec, r)
	})
}

// ---------------------------------------------------------------------------
// sessions (server side, in memory)
// ---------------------------------------------------------------------------

type Sess struct {
	ID        string
	UID       int
	Username  string
	Role      string
	CompanyID int
}

var (
	sessions = map[string]*Sess{}
	sessMu   sync.Mutex
)

func validSID(s string) bool {
	if len(s) < 8 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func loadSession(w http.ResponseWriter, r *http.Request) *Sess {
	sessMu.Lock()
	defer sessMu.Unlock()
	if c, err := r.Cookie("MCSESS"); err == nil && validSID(c.Value) {
		if s, ok := sessions[c.Value]; ok {
			return s
		}
		// unknown but well-formed identifier: adopt it
		s := &Sess{ID: c.Value}
		sessions[c.Value] = s
		return s
	}
	id := randHex(16)
	s := &Sess{ID: id}
	sessions[id] = s
	http.SetCookie(w, &http.Cookie{Name: "MCSESS", Value: id, Path: "/"})
	return s
}

func sessionOf(r *http.Request) *Sess {
	s, _ := r.Context().Value(sessKey).(*Sess)
	if s == nil {
		s = &Sess{}
	}
	return s
}

// ---------------------------------------------------------------------------
// internal service bound to loopback only
// ---------------------------------------------------------------------------

func internalService() {
	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"service": "meridian-internal",
			"routes":  []string{"/internal/status", "/internal/credentials", "/internal/ops-token"},
		})
	})
	m.HandleFunc("/internal/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "node": "mc-app-01", "queue": 14})
	})
	m.HandleFunc("/internal/credentials", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"carrier_edi": map[string]string{"host": "edi.carrier-hub.example", "user": "edi_sync", "password": "Edi$ync-7731-Rq"},
			"customs":     map[string]string{"host": "filer.customs-gw.example", "user": "mc_filer", "password": "Cstm#Filer-2291"},
		})
	})
	m.HandleFunc("/internal/ops-token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ops_service_token": "mcops_9f3a1c77d2e84b60a5d1"})
	})
	if err := http.ListenAndServe("127.0.0.1:9090", m); err != nil {
		appLog.Printf("internal service: %v", err)
	}
}

// neutered file system: static assets are served without directory listings
type noListFS struct{ fs http.FileSystem }

func (n noListFS) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		idx := filepath.Join(name, "index.html")
		if _, err := n.fs.Open(idx); err != nil {
			f.Close()
			return nil, os.ErrNotExist
		}
	}
	return f, nil
}
