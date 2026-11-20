package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func routes(mux *http.ServeMux) {
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(noListFS{http.Dir("/app/static")})))
	// report exports and customer uploads are plain file shares
	mux.Handle("/exports/", http.StripPrefix("/exports/", http.FileServer(http.Dir("/app/exports"))))
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("/app/uploads"))))

	mux.HandleFunc("GET /{$}", hHome)
	mux.HandleFunc("GET /robots.txt", hRobots)
	mux.HandleFunc("GET /track", hTrack)
	mux.HandleFunc("GET /ports", hPorts)
	mux.HandleFunc("GET /help", hHelp)

	mux.HandleFunc("GET /login", hLoginPage)
	mux.HandleFunc("POST /login", hLogin)
	mux.HandleFunc("GET /logout", hLogout)
	mux.HandleFunc("GET /register", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, "register", Page{"Title": "Create account", "Page": "register"})
	})
	mux.HandleFunc("GET /forgot", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, "forgot", Page{"Title": "Forgot password", "Page": "forgot"})
	})
	mux.HandleFunc("POST /forgot", hForgot)
	mux.HandleFunc("POST /forgot/verify", hForgotVerify)

	mux.HandleFunc("GET /dashboard", needLogin(hDashboard))
	mux.HandleFunc("GET /shipments", needLogin(hShipments))
	mux.HandleFunc("GET /shipments/view", needLogin(hShipmentView))
	mux.HandleFunc("POST /shipments/note", needLogin(hShipmentNote))
	mux.HandleFunc("GET /quote", needLogin(hQuote))
	mux.HandleFunc("POST /quote/book", needLogin(hBook))
	mux.HandleFunc("GET /documents", needLogin(hDocuments))
	mux.HandleFunc("GET /documents/download", needLogin(hDownload))
	mux.HandleFunc("POST /documents/upload", needLogin(hUpload))
	mux.HandleFunc("GET /account", needLogin(hAccount))
	mux.HandleFunc("/account/contact", needLogin(hContact))
	mux.HandleFunc("GET /account/invoices", needLogin(hInvoices))
	mux.HandleFunc("GET /account/invoices/view", needLogin(hInvoiceView))
	mux.HandleFunc("GET /integrations", needLogin(func(w http.ResponseWriter, r *http.Request, a *Auth) {
		render(w, r, "integrations", Page{"Title": "Integrations", "Page": "integrations"})
	}))

	mux.HandleFunc("GET /ops", needStaff(hOps))
	mux.HandleFunc("GET /ops/tools", needStaff(hToolsPage))
	mux.HandleFunc("POST /ops/tools", needStaff(hToolsRun))
	mux.HandleFunc("GET /ops/logs", needStaff(hLogs))
	mux.HandleFunc("GET /admin/users", needAdmin(hAdminUsers))

	apiRoutes(mux)
}

// ---------------------------------------------------------------------------
// rendering helpers
// ---------------------------------------------------------------------------

func render(w http.ResponseWriter, r *http.Request, name string, data Page) {
	if data == nil {
		data = Page{}
	}
	data["Auth"] = authUser(r)
	t, ok := pages[name]
	if !ok {
		http.Error(w, "template missing", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		appLog.Printf("template error %s: %v", name, err)
	}
}

func renderStatus(w http.ResponseWriter, r *http.Request, code int, title, msg string) {
	w.WriteHeader(code)
	render(w, r, "error", Page{"Title": title, "Msg": msg, "Code": code})
}

func jsonOut(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------
// public pages
// ---------------------------------------------------------------------------

func hHome(w http.ResponseWriter, r *http.Request) {
	render(w, r, "home", Page{"Title": "Global freight, tracked", "Page": "home"})
}

func hRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, "User-agent: *\nDisallow: /ops/\nDisallow: /admin/\nDisallow: /exports/\nDisallow: /account/\n")
}

type Shipment struct {
	ID           int
	Ref          string
	Company      string
	CompanyID    int
	Mode         string
	Origin, Dest string
	Status       string
	Container    string
	WeightKG     int
	ETA          string
	Instructions string
}

type Event struct{ When, Where, What string }

const shipCols = `s.id, s.ref, c.name, s.company_id, s.mode, s.origin_code, s.dest_code, s.status,
	COALESCE(s.container_no,''), s.weight_kg, COALESCE(DATE_FORMAT(s.eta,'%Y-%m-%d'),''), COALESCE(s.instructions,'')`

func scanShip(sc interface{ Scan(...any) error }) (Shipment, error) {
	var s Shipment
	err := sc.Scan(&s.ID, &s.Ref, &s.Company, &s.CompanyID, &s.Mode, &s.Origin, &s.Dest, &s.Status, &s.Container, &s.WeightKG, &s.ETA, &s.Instructions)
	return s, err
}

func eventsFor(id int) []Event {
	var out []Event
	rows, err := db.Query("SELECT DATE_FORMAT(event_time,'%Y-%m-%d %H:%i'), location, description FROM shipment_events WHERE shipment_id=? ORDER BY event_time DESC", id)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var e Event
		rows.Scan(&e.When, &e.Where, &e.What)
		out = append(out, e)
	}
	return out
}

func hTrack(w http.ResponseWriter, r *http.Request) {
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	data := Page{"Title": "Track a shipment", "Page": "track", "Ref": ref}
	if ref != "" {
		s, err := scanShip(db.QueryRow("SELECT "+shipCols+" FROM shipments s JOIN companies c ON c.id=s.company_id WHERE s.ref=?", ref))
		if err == nil {
			data["Found"] = true
			data["S"] = s
			data["Events"] = eventsFor(s.ID)
		} else {
			data["NotFound"] = true
		}
	}
	render(w, r, "track", data)
}

func hPorts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	data := Page{"Title": "Port directory", "Page": "ports", "Q": q}
	query := "SELECT code, name, country, region, notes FROM ports WHERE name LIKE '%" + q + "%' OR code LIKE '%" + q + "%' OR country LIKE '%" + q + "%' ORDER BY name"
	rows, err := db.Query(query)
	if err != nil {
		appLog.Printf("ports query failed: %v", err)
		data["Failed"] = true
		render(w, r, "ports", data)
		return
	}
	defer rows.Close()
	var list [][]string
	for rows.Next() {
		var c [5]sql.NullString
		if err := rows.Scan(&c[0], &c[1], &c[2], &c[3], &c[4]); err != nil {
			continue
		}
		list = append(list, []string{c[0].String, c[1].String, c[2].String, c[3].String, c[4].String})
	}
	data["Rows"] = list
	render(w, r, "ports", data)
}

type article struct{ Title, Body string }

var kb = []article{
	{"How do I track a container?", "Enter your shipment reference (format MC-YYYY-NNNNN) on the tracking page. No login is required."},
	{"What documents are needed for customs clearance?", "A commercial invoice, packing list and bill of lading are required for every consignment."},
	{"How is volumetric weight calculated?", "For air freight we divide the cubic volume in centimetres by 6000 to obtain chargeable kilograms."},
	{"Can I change delivery instructions after booking?", "Yes. Open the shipment from your dashboard and edit the delivery instructions until the status becomes In Transit."},
	{"How do I reset my password?", "Use the 'Forgot password' link on the sign-in page. A 4-digit PIN is sent to your registered e-mail address."},
	{"What are demurrage and detention charges?", "Charges applied by the carrier when a container is kept beyond the free time at port or outside the terminal."},
	{"How do webhooks work?", "Under Integrations you can register an endpoint to receive shipment status callbacks. Use Test to verify reachability."},
}

func hHelp(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	data := Page{"Title": "Help centre", "Page": "help", "Q": q}
	var res []article
	for _, a := range kb {
		if q == "" || strings.Contains(strings.ToLower(a.Title+a.Body), strings.ToLower(q)) {
			res = append(res, a)
		}
	}
	data["Results"] = res
	if q != "" && len(res) == 0 {
		// message echoes the search term
		data["NoneMsg"] = template.HTML("No articles found for <strong>" + q + "</strong>.")
	}
	render(w, r, "help", data)
}

// ---------------------------------------------------------------------------
// authentication
// ---------------------------------------------------------------------------

func hLoginPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, "login", Page{"Title": "Sign in", "Page": "login", "Next": r.URL.Query().Get("next")})
}

func safeNext(n string) string {
	if strings.HasPrefix(n, "/") && !strings.HasPrefix(n, "//") {
		return n
	}
	return ""
}

func hLogin(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	user := strings.TrimSpace(r.PostFormValue("username"))
	pass := r.PostFormValue("password")
	next := safeNext(r.PostFormValue("next"))
	var id, cid int
	var role, hash string
	err := db.QueryRow("SELECT id, company_id, role, password_hash FROM users WHERE username=?", user).Scan(&id, &cid, &role, &hash)
	ip := r.RemoteAddr
	if err != nil || hash != hashPW(pass) {
		appLog.Printf("auth login user=%s password=%s result=fail ip=%s", user, pass, ip)
		w.WriteHeader(401)
		render(w, r, "login", Page{"Title": "Sign in", "Page": "login", "Error": "Invalid username or password.", "Next": next})
		return
	}
	appLog.Printf("auth login user=%s password=%s result=ok ip=%s", user, pass, ip)
	s := sessionOf(r)
	sessMu.Lock()
	s.UID, s.Username, s.Role, s.CompanyID = id, user, role, cid
	sessMu.Unlock()
	if next != "" {
		http.Redirect(w, r, next, http.StatusFound)
		return
	}
	if role == "staff" || role == "admin" {
		http.Redirect(w, r, "/ops", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func hLogout(w http.ResponseWriter, r *http.Request) {
	s := sessionOf(r)
	sessMu.Lock()
	delete(sessions, s.ID)
	sessMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "MCSESS", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusFound)
}

func maskEmail(e string) string {
	at := strings.Index(e, "@")
	if at < 2 {
		return e
	}
	return e[:1] + strings.Repeat("*", at-1) + e[at:]
}

func hForgot(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	user := strings.TrimSpace(r.PostFormValue("username"))
	var id int
	var email string
	err := db.QueryRow("SELECT id, email FROM users WHERE username=?", user).Scan(&id, &email)
	if err != nil {
		render(w, r, "forgot", Page{"Title": "Forgot password", "Page": "forgot", "Error": "No account exists with that username."})
		return
	}
	pin := randPIN()
	db.Exec("UPDATE users SET reset_pin=?, reset_expires=DATE_ADD(NOW(), INTERVAL 30 MINUTE) WHERE id=?", pin, id)
	appLog.Printf("auth reset-pin issued user=%s pin=%s email=%s", user, pin, email)
	render(w, r, "reset", Page{"Title": "Enter PIN", "Page": "reset", "Username": user, "Masked": maskEmail(email)})
}

func hForgotVerify(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	user := strings.TrimSpace(r.PostFormValue("username"))
	pin := strings.TrimSpace(r.PostFormValue("pin"))
	npw := r.PostFormValue("new_password")
	var id int
	var dbpin sql.NullString
	var exp sql.NullTime
	err := db.QueryRow("SELECT id, reset_pin, reset_expires FROM users WHERE username=?", user).Scan(&id, &dbpin, &exp)
	data := Page{"Title": "Enter PIN", "Page": "reset", "Username": user, "Masked": ""}
	if err != nil || !dbpin.Valid || dbpin.String != pin || !exp.Valid || time.Now().After(exp.Time) {
		appLog.Printf("auth reset-pin failed user=%s pin=%s", user, pin)
		data["Error"] = "The PIN is incorrect or has expired."
		render(w, r, "reset", data)
		return
	}
	if len(npw) < 6 {
		data["Error"] = "Password must be at least 6 characters."
		render(w, r, "reset", data)
		return
	}
	db.Exec("UPDATE users SET password_hash=?, reset_pin=NULL, reset_expires=NULL WHERE id=?", hashPW(npw), id)
	appLog.Printf("auth password-reset user=%s new_password=%s", user, npw)
	render(w, r, "login", Page{"Title": "Sign in", "Page": "login", "Notice": "Password updated. You can sign in now."})
}

// ---------------------------------------------------------------------------
// customer portal
// ---------------------------------------------------------------------------

func hDashboard(w http.ResponseWriter, r *http.Request, a *Auth) {
	var active, docs, openInv int
	var openAmt sql.NullInt64
	db.QueryRow("SELECT COUNT(*) FROM shipments WHERE company_id=? AND status<>'Delivered'", a.CompanyID).Scan(&active)
	db.QueryRow("SELECT COUNT(*) FROM documents WHERE company_id=?", a.CompanyID).Scan(&docs)
	db.QueryRow("SELECT COUNT(*), SUM(amount_cents) FROM invoices WHERE company_id=? AND status='Open'", a.CompanyID).Scan(&openInv, &openAmt)
	var recent []Shipment
	rows, err := db.Query("SELECT "+shipCols+" FROM shipments s JOIN companies c ON c.id=s.company_id WHERE s.company_id=? ORDER BY s.id DESC LIMIT 5", a.CompanyID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			if s, e := scanShip(rows); e == nil {
				recent = append(recent, s)
			}
		}
	}
	var cname, fname string
	db.QueryRow("SELECT c.name, u.full_name FROM users u JOIN companies c ON c.id=u.company_id WHERE u.id=?", a.UID).Scan(&cname, &fname)
	render(w, r, "dashboard", Page{"Title": "Dashboard", "Page": "dashboard", "Active": active, "Docs": docs,
		"OpenInv": openInv, "OpenAmt": int(openAmt.Int64), "Recent": recent, "Company": cname, "FullName": fname})
}

func hShipments(w http.ResponseWriter, r *http.Request, a *Auth) {
	status := r.URL.Query().Get("status")
	var list []Shipment
	rows, err := db.Query("SELECT "+shipCols+" FROM shipments s JOIN companies c ON c.id=s.company_id WHERE s.company_id=? AND (?='' OR s.status=?) ORDER BY s.id DESC", a.CompanyID, status, status)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			if s, e := scanShip(rows); e == nil {
				list = append(list, s)
			}
		}
	}
	render(w, r, "shipments", Page{"Title": "My shipments", "Page": "shipments", "List": list, "Status": status})
}

func hShipmentView(w http.ResponseWriter, r *http.Request, a *Auth) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	s, err := scanShip(db.QueryRow("SELECT "+shipCols+" FROM shipments s JOIN companies c ON c.id=s.company_id WHERE s.id=?", id))
	if err != nil || (s.CompanyID != a.CompanyID && !a.IsStaff()) {
		renderStatus(w, r, 404, "Not found", "That shipment could not be found.")
		return
	}
	render(w, r, "shipment", Page{"Title": s.Ref, "Page": "shipment", "S": s, "Events": eventsFor(s.ID),
		"Instr": template.HTML(s.Instructions)})
}

func hShipmentNote(w http.ResponseWriter, r *http.Request, a *Auth) {
	r.ParseForm()
	id, _ := strconv.Atoi(r.PostFormValue("id"))
	note := r.PostFormValue("instructions")
	res, err := db.Exec("UPDATE shipments SET instructions=? WHERE id=? AND company_id=?", note, id, a.CompanyID)
	if err != nil {
		renderStatus(w, r, 500, "Error", "Could not save instructions.")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// RowsAffected is 0 when unchanged as well; fall through to redirect
	}
	http.Redirect(w, r, "/shipments/view?id="+strconv.Itoa(id), http.StatusFound)
}

type Port struct{ Code, Name string }

func hQuote(w http.ResponseWriter, r *http.Request, a *Auth) {
	var ports []Port
	rows, err := db.Query("SELECT code, name FROM ports ORDER BY name")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p Port
			rows.Scan(&p.Code, &p.Name)
			ports = append(ports, p)
		}
	}
	render(w, r, "quote", Page{"Title": "Get a quote", "Page": "quote", "Ports": ports})
}

func hBook(w http.ResponseWriter, r *http.Request, a *Auth) {
	r.ParseForm()
	origin := r.PostFormValue("origin")
	dest := r.PostFormValue("dest")
	mode := r.PostFormValue("mode")
	weight, _ := strconv.Atoi(r.PostFormValue("weight"))
	rate, err := strconv.Atoi(r.PostFormValue("rate_cents"))
	instr := r.PostFormValue("instructions")
	if err != nil || origin == "" || dest == "" || weight <= 0 {
		renderStatus(w, r, 400, "Invalid booking", "Please complete all fields of the booking form.")
		return
	}
	ref := fmt.Sprintf("MC-%d-%05d", time.Now().Year(), 20000+rand.Intn(70000))
	res, err := db.Exec(`INSERT INTO shipments (ref, company_id, mode, origin_code, dest_code, status, weight_kg, eta, instructions, created_by)
		VALUES (?,?,?,?,?,'Booked',?, DATE_ADD(CURDATE(), INTERVAL 21 DAY), ?, ?)`, ref, a.CompanyID, mode, origin, dest, weight, instr, a.UID)
	if err != nil {
		renderStatus(w, r, 500, "Booking failed", "We could not create the booking. Please try again.")
		return
	}
	sid, _ := res.LastInsertId()
	db.Exec("INSERT INTO invoices (company_id, shipment_id, invoice_no, amount_cents, currency, status, due_date, billed_to) VALUES (?,?,?,?, 'USD','Open', DATE_ADD(CURDATE(), INTERVAL 30 DAY), (SELECT email FROM users WHERE id=?))",
		a.CompanyID, sid, fmt.Sprintf("INV-%d-%05d", time.Now().Year(), 40000+rand.Intn(50000)), rate, a.UID)
	db.Exec("INSERT INTO shipment_events (shipment_id, event_time, location, description) VALUES (?, NOW(), ?, 'Booking confirmed')", sid, origin)
	http.Redirect(w, r, "/shipments/view?id="+strconv.FormatInt(sid, 10), http.StatusFound)
}

type Doc struct {
	Title, File, Source, Uploaded string
}

func hDocuments(w http.ResponseWriter, r *http.Request, a *Auth) {
	var docs []Doc
	rows, err := db.Query("SELECT title, filename, source, DATE_FORMAT(uploaded_at,'%Y-%m-%d') FROM documents WHERE company_id=? ORDER BY id DESC", a.CompanyID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d Doc
			rows.Scan(&d.Title, &d.File, &d.Source, &d.Uploaded)
			docs = append(docs, d)
		}
	}
	render(w, r, "documents", Page{"Title": "Documents", "Page": "documents", "Docs": docs})
}

func hDownload(w http.ResponseWriter, r *http.Request, a *Auth) {
	name := r.URL.Query().Get("name")
	p := filepath.Join("/app/data/docs", name)
	b, err := os.ReadFile(p)
	if err != nil {
		renderStatus(w, r, 404, "Document not found", "The requested document does not exist.")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="`+filepath.Base(name)+`"`)
	w.Write(b)
}

func hUpload(w http.ResponseWriter, r *http.Request, a *Auth) {
	r.ParseMultipartForm(8 << 20)
	f, hdr, err := r.FormFile("document")
	if err != nil {
		renderStatus(w, r, 400, "Upload failed", "No file received.")
		return
	}
	defer f.Close()
	title := strings.TrimSpace(r.FormValue("title"))
	name := filepath.Base(hdr.Filename)
	if name == "." || name == "/" || name == "" {
		renderStatus(w, r, 400, "Upload failed", "Invalid file name.")
		return
	}
	out, err := os.Create(filepath.Join("/app/uploads", name))
	if err != nil {
		renderStatus(w, r, 500, "Upload failed", "Could not store the file.")
		return
	}
	io.Copy(out, io.LimitReader(f, 8<<20))
	out.Close()
	if title == "" {
		title = name
	}
	db.Exec("INSERT INTO documents (company_id, title, filename, source, uploaded_at) VALUES (?,?,?, 'upload', NOW())", a.CompanyID, title, name)
	http.Redirect(w, r, "/documents", http.StatusFound)
}

func hAccount(w http.ResponseWriter, r *http.Request, a *Auth) {
	var email, phone, full, cname string
	db.QueryRow("SELECT u.email, COALESCE(u.phone,''), u.full_name, c.name FROM users u JOIN companies c ON c.id=u.company_id WHERE u.id=?", a.UID).Scan(&email, &phone, &full, &cname)
	render(w, r, "account", Page{"Title": "My account", "Page": "account", "Email": email, "Phone": phone, "FullName": full, "Company": cname,
		"Notice": r.URL.Query().Get("saved")})
}

func hContact(w http.ResponseWriter, r *http.Request, a *Auth) {
	// accepts both GET and POST submissions
	email := strings.TrimSpace(r.FormValue("email"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	if email != "" {
		db.Exec("UPDATE users SET email=? WHERE id=?", email, a.UID)
		appLog.Printf("account contact-update user=%s email=%s", a.Username, email)
	}
	if phone != "" {
		db.Exec("UPDATE users SET phone=? WHERE id=?", phone, a.UID)
	}
	http.Redirect(w, r, "/account?saved=1", http.StatusFound)
}

type Invoice struct {
	ID                                    int
	No, Company, Status, Due, Ship, Billed string
	Amount                                int
	Currency                              string
}

func hInvoices(w http.ResponseWriter, r *http.Request, a *Auth) {
	var list []Invoice
	rows, err := db.Query(`SELECT i.id, i.invoice_no, c.name, i.status, DATE_FORMAT(i.due_date,'%Y-%m-%d'), i.amount_cents, i.currency
		FROM invoices i JOIN companies c ON c.id=i.company_id WHERE i.company_id=? ORDER BY i.id DESC`, a.CompanyID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var v Invoice
			rows.Scan(&v.ID, &v.No, &v.Company, &v.Status, &v.Due, &v.Amount, &v.Currency)
			list = append(list, v)
		}
	}
	render(w, r, "invoices", Page{"Title": "Invoices", "Page": "invoices", "List": list})
}

func hInvoiceView(w http.ResponseWriter, r *http.Request, a *Auth) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	var v Invoice
	err := db.QueryRow(`SELECT i.id, i.invoice_no, c.name, i.status, DATE_FORMAT(i.due_date,'%Y-%m-%d'), i.amount_cents, i.currency,
		COALESCE(s.ref,''), COALESCE(i.billed_to,'') FROM invoices i JOIN companies c ON c.id=i.company_id
		LEFT JOIN shipments s ON s.id=i.shipment_id WHERE i.id=?`, id).Scan(&v.ID, &v.No, &v.Company, &v.Status, &v.Due, &v.Amount, &v.Currency, &v.Ship, &v.Billed)
	if err != nil {
		renderStatus(w, r, 404, "Not found", "Invoice not found.")
		return
	}
	render(w, r, "invoice", Page{"Title": v.No, "Page": "invoice", "V": v})
}

// ---------------------------------------------------------------------------
// operations console (staff)
// ---------------------------------------------------------------------------

func hOps(w http.ResponseWriter, r *http.Request, a *Auth) {
	var total, hold, late, openTick int
	db.QueryRow("SELECT COUNT(*) FROM shipments").Scan(&total)
	db.QueryRow("SELECT COUNT(*) FROM shipments WHERE status='Customs Hold'").Scan(&hold)
	db.QueryRow("SELECT COUNT(*) FROM shipments WHERE status='Delayed'").Scan(&late)
	openTick = 0
	var list []Shipment
	rows, err := db.Query("SELECT " + shipCols + " FROM shipments s JOIN companies c ON c.id=s.company_id ORDER BY s.id DESC LIMIT 25")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			if s, e := scanShip(rows); e == nil {
				list = append(list, s)
			}
		}
	}
	// the dispatch queue shows customer delivery instructions verbatim
	type row struct {
		S     Shipment
		Instr template.HTML
	}
	var qrows []row
	for _, s := range list {
		qrows = append(qrows, row{s, template.HTML(s.Instructions)})
	}
	render(w, r, "ops", Page{"Title": "Operations console", "Page": "ops", "Total": total, "Hold": hold, "Late": late, "Tick": openTick, "Rows": qrows})
}

func hToolsPage(w http.ResponseWriter, r *http.Request, a *Auth) {
	render(w, r, "ops_tools", Page{"Title": "Network tools", "Page": "ops_tools"})
}

func hToolsRun(w http.ResponseWriter, r *http.Request, a *Auth) {
	r.ParseForm()
	host := r.PostFormValue("host")
	out, _ := exec.Command("sh", "-c", "ping -c 2 -W 2 "+host).CombinedOutput()
	render(w, r, "ops_tools", Page{"Title": "Network tools", "Page": "ops_tools", "Host": host, "Output": string(out), "Ran": true})
}

func hLogs(w http.ResponseWriter, r *http.Request, a *Auth) {
	b, _ := os.ReadFile("/app/logs/app.log")
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > 300 {
		lines = lines[len(lines)-300:]
	}
	render(w, r, "ops_logs", Page{"Title": "Application log", "Page": "ops_logs", "Log": strings.Join(lines, "\n")})
}

type UserRow struct {
	ID                                  int
	Username, Email, Role, Company, Hash string
}

func hAdminUsers(w http.ResponseWriter, r *http.Request, a *Auth) {
	var list []UserRow
	rows, err := db.Query("SELECT u.id, u.username, u.email, u.role, c.name, u.password_hash FROM users u JOIN companies c ON c.id=u.company_id ORDER BY u.id")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var u UserRow
			rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.Company, &u.Hash)
			list = append(list, u)
		}
	}
	render(w, r, "admin_users", Page{"Title": "User administration", "Page": "admin_users", "List": list})
}

var _ = url.QueryEscape
