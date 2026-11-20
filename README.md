# CHRIS-LAB — Meridian Cargo Portal (black-box web application PT lab)

A deliberately vulnerable freight-forwarding customer portal, built to simulate an internal
web-application penetration-test exam. Treat it as a **real client engagement**: you get a URL, nothing else.

> **Insecure by design.** Run it only inside your own isolated lab/VM. Never expose it to a
> shared network, the internet, or a machine that holds real data.

---

## 1. Engagement brief

| Item | Value |
|---|---|
| Client | Meridian Cargo Ltd (fictional) |
| Target | `http://192.168.255.133:8090/` |
| Type | Black-box web application penetration test |
| Credentials provided | **None.** The portal has self-registration. |
| Expected findings | **27** distinct vulnerabilities |
| Time suggestion | 6–8 hours (exam pace) |

### In scope
* Everything reachable over HTTP on port **8090** of the target host.
* Anything you can reach *through* the application (that is part of the test).

### Out of scope
* Denial of service / resource exhaustion, mass brute-forcing beyond what is needed to prove a finding.
* Attacking the Docker host, SSH, or other ports/containers directly.
* Reading the lab's own source files (`app/`, `Dockerfile`, `db/`, `docker-compose.yml`) — that would turn it into a white-box exercise and spoil the challenge. Use `SOLUTIONS.md` only **after** you are finished.

---

## 2. What the findings look like

The 27 findings cover the same vulnerability families found across the PolyVuln labs, but they are
placed in **different features** and behave differently — do not expect a 1:1 endpoint match.

| OWASP Top 10:2025 | Findings |
|---|---|
| A01 Broken Access Control | 6 |
| A02 Security Misconfiguration | 3 |
| A03 Software Supply Chain Failures | 1 |
| A04 Cryptographic Failures | 2 |
| A05 Injection | 5 |
| A06 Insecure Design | 2 |
| A07 Authentication Failures | 4 |
| A08 Software or Data Integrity Failures | 2 |
| A09 Security Logging & Alerting Failures | 1 |
| A10 Mishandling of Exceptional Conditions | 1 |
| **Total** | **27** |

Notes:
* There is **no** Oracle, PostgreSQL or NoSQL back end in this lab.
* Some findings chain together (a low-impact bug often unlocks a higher-impact one).
* Some features are only reachable with a higher-privileged account — finding a way in is part of the exercise.
* Every finding is reproducible with a browser + Burp Suite (or curl). No special exploit tooling is required.

---

## 3. Suggested methodology

1. Map the application (spider, forced browsing, JS files, robots.txt, response headers).
2. Register an account and enumerate every authenticated feature.
3. Test each input for injection, each object reference for access control, each state change for CSRF.
4. Review authentication, session handling, password recovery and API token handling.
5. Look at what the application leaks: errors, headers, comments, diagnostics, logs, static components.
6. Chain findings to reach the staff and administrator areas.

## 4. Deliverable template

For each finding record: **title · severity (CVSS) · OWASP/CWE · affected URL & parameter · steps to reproduce ·
evidence (request/response or screenshot) · impact · remediation.**

Keep a running checklist (1–27) and mark each one when proven.

---

## 5. Running the lab

Requirements: Docker Engine with the Compose plugin, ~1 GB RAM, internet access on first build.

```bash
cd /home/kali/PTLAB/CHRIS-LAB
docker compose up -d --build      # build + start (first start takes ~30–60 s for the database)
./reset.sh                        # wipe all data and restart from the pristine seed state
docker compose down -v            # stop and delete everything
```

The application listens on host port **8090** (change the left side of `8090:8080` in `docker-compose.yml`
if the port is taken). Container logs: `docker logs -f meridian-web`.

> Run `./reset.sh` whenever you want a clean slate (registered users, uploads, changed passwords and
> logs are all discarded).

## 6. Files

| File | Purpose |
|---|---|
| `README.md` | This document (safe to read before testing) |
| `SOLUTIONS.md` | Full answer key — **spoilers**, including the technology stack |
| `reset.sh` | Reset the lab to its initial state |
| `docker-compose.yml`, `Dockerfile`, `db/`, `app/` | Lab implementation — do not read before you finish |

## 7. Legal

For training and authorised testing only. All company names, people, e-mail addresses and data in the
lab are fictional.
