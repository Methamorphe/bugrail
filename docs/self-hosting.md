# Self-hosting Bugrail

This guide walks you through deploying Bugrail on a Linux VPS so you have a real public DSN
to point your SDKs at. The result: a single binary behind nginx, running as a systemd service.

Estimated time: 20 minutes on a fresh Debian/Ubuntu server.

---

## Requirements

- A VPS with at least 512 MB RAM and 2 GB disk (a 5 €/month Hetzner CX11 or equivalent is fine)
- Go 1.23+ **or** a pre-built binary from the releases page
- Root or sudo access on the server

A domain name is **not required**. See [section 6](#6-expose-bugrail-choose-one) for your options.

---

## 1. Build the binary

**Option A — build on the server** (requires Go):

```bash
git clone https://github.com/Methamorphe/bugrail.git
cd bugrail
go build -o /usr/local/bin/bugrail ./cmd/bugrail
```

**Option B — cross-compile locally and upload**:

```bash
# On your Mac (Apple Silicon → Linux amd64)
GOOS=linux GOARCH=amd64 go build -o bugrail-linux ./cmd/bugrail
scp bugrail-linux user@your-server:/usr/local/bin/bugrail
ssh user@your-server "chmod +x /usr/local/bin/bugrail"
```

Verify:

```bash
bugrail --help
# usage: bugrail <init|serve|migrate>
```

---

## 2. Create a system user and data directory

```bash
useradd --system --no-create-home --shell /usr/sbin/nologin bugrail
mkdir -p /var/lib/bugrail
chown bugrail:bugrail /var/lib/bugrail
chmod 750 /var/lib/bugrail
```

---

## 3. Configure environment variables

Bugrail is configured entirely through environment variables. Create a file that systemd will load:

```bash
cat > /etc/bugrail.env << 'EOF'
# Directory where bugrail stores its SQLite database (and attachments).
BUGRAIL_DATA_DIR=/var/lib/bugrail

# The address the HTTP server listens on (nginx will proxy to it).
BUGRAIL_LISTEN_ADDR=127.0.0.1:8080

# Maximum events per minute accepted per project before rate limiting kicks in.
# Default: 1000. Lower this if you want to protect disk space.
BUGRAIL_RATE_LIMIT_PER_PROJECT=1000

# Optional: use PostgreSQL instead of SQLite (see the PostgreSQL section).
# BUGRAIL_DATABASE_URL=postgres://user:pass@localhost/bugrail?sslmode=disable
EOF
chmod 600 /etc/bugrail.env
```

---

## 4. Initialize the database

Run `bugrail init` once to create the database, admin account, and first project.
Do this as the `bugrail` user so file ownership is correct:

```bash
sudo -u bugrail env $(cat /etc/bugrail.env | xargs) bugrail init \
  --admin-email you@example.com \
  --admin-password "choose-a-strong-password" \
  --org-name "My Org" \
  --project-name "My App"
```

The command prints a DSN. **Ignore the hostname for now** — it shows `localhost:8080`
because the binary doesn't know your public address yet. You will reconstruct the DSN in
section 7 once you know your public URL.

Example output:

```
Admin: you@example.com
Project: My App
DSN: http://<key>@localhost:8080/1
```

Note the `<key>` — you will need it in section 7.

---

## 5. Create a systemd service

```bash
cat > /etc/systemd/system/bugrail.service << 'EOF'
[Unit]
Description=Bugrail error tracker
After=network.target

[Service]
User=bugrail
Group=bugrail
EnvironmentFile=/etc/bugrail.env
ExecStart=/usr/local/bin/bugrail serve
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/bugrail

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now bugrail
systemctl status bugrail
```

Check that it started cleanly:

```bash
journalctl -u bugrail -n 30
# Should show: "starting bugrail" with listen_addr=127.0.0.1:8080
```

---

## 6. Expose Bugrail — choose one

Three options, in order of simplicity. Pick the one that fits your situation.

---

### Option A — HTTP only (simplest, no domain needed)

Skip nginx entirely. Open port 8080 directly, change `BUGRAIL_LISTEN_ADDR` to listen on all
interfaces, and access Bugrail via its IP.

```bash
# In /etc/bugrail.env, change:
BUGRAIL_LISTEN_ADDR=0.0.0.0:8080
```

Restart the service and open the firewall:

```bash
systemctl restart bugrail
ufw allow 22/tcp
ufw allow 8080/tcp
ufw enable
```

Your public URL is `http://<server-ip>:8080` and your DSN will look like:

```
http://<key>@<server-ip>:8080/1
```

**When to use this:** local testing, a private network, or a quick proof of concept.
Traffic is unencrypted — don't use this if the server is reachable from the internet and you
care about the DSN key leaking.

---

### Option B — HTTPS with sslip.io (no domain purchase needed)

`sslip.io` is a free public DNS service. Any address like `1-2-3-4.sslip.io` resolves to
the IP `1.2.3.4`. Let's Encrypt can issue a certificate for it, so you get valid HTTPS
without buying a domain.

Replace dots with dashes in your server IP to build the hostname:

```
Server IP: 1.2.3.4  →  hostname: bugrail.1-2-3-4.sslip.io
```

Install nginx and Certbot:

```bash
apt install -y nginx certbot python3-certbot-nginx
```

Create a site config (replace `1-2-3-4` with your actual IP):

```bash
cat > /etc/nginx/sites-available/bugrail << 'EOF'
server {
    listen 80;
    server_name bugrail.1-2-3-4.sslip.io;

    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        client_max_body_size 50m;
    }
}
EOF

ln -s /etc/nginx/sites-available/bugrail /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx
```

Obtain the certificate:

```bash
certbot --nginx -d bugrail.1-2-3-4.sslip.io
```

Open the firewall:

```bash
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable
```

Your public URL is `https://bugrail.1-2-3-4.sslip.io`.

**When to use this:** when you want real HTTPS without paying for a domain. The sslip.io
service is maintained by the Cloud Foundry community and has been reliable for years, but
it's a third-party dependency — if it goes down your hostname stops resolving. For anything
critical, buy a domain (Option C).

---

### Option C — HTTPS with your own domain

Same as Option B but with a real domain you control. Point an A record at your server IP,
then:

```bash
apt install -y nginx certbot python3-certbot-nginx

cat > /etc/nginx/sites-available/bugrail << 'EOF'
server {
    listen 80;
    server_name bugrail.example.com;

    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        client_max_body_size 50m;
    }
}
EOF

ln -s /etc/nginx/sites-available/bugrail /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx
certbot --nginx -d bugrail.example.com
```

Open the firewall:

```bash
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable
```

Test certificate renewal: `certbot renew --dry-run`.

---

## 7. Construct your real DSN

The DSN format is:

```
<scheme>://<key>@<your-host>/<project-id>
```

From the `bugrail init` output, take the `<key>`. Then substitute your public address:

| Setup | DSN |
|---|---|
| Option A (HTTP + IP) | `http://<key>@1.2.3.4:8080/1` |
| Option B (sslip.io) | `https://<key>@bugrail.1-2-3-4.sslip.io/1` |
| Option C (own domain) | `https://<key>@bugrail.example.com/1` |

The project ID is `1` for the first project created by `bugrail init`.

---

## 8. Point your SDK at Bugrail

Replace the Sentry DSN in your application. No other change needed.

```js
// Node / JavaScript
Sentry.init({ dsn: "https://<key>@bugrail.1-2-3-4.sslip.io/1" });
```

```python
# Python
sentry_sdk.init(dsn="https://<key>@bugrail.1-2-3-4.sslip.io/1")
```

```php
// PHP
\Sentry\init(['dsn' => 'https://<key>@bugrail.1-2-3-4.sslip.io/1']);
```

```go
// Go
sentry.Init(sentry.ClientOptions{Dsn: "https://<key>@bugrail.1-2-3-4.sslip.io/1"})
```

---

## 9. Verify end-to-end

Trigger a test error in your app, or use the seed tool from the repo:

```bash
go run ./cmd/seed https://<key>@bugrail.1-2-3-4.sslip.io/1
```

Open your public URL in a browser, log in, and check the dashboard.

---

## Optional: PostgreSQL instead of SQLite

SQLite is the right default for most deployments. Switch to PostgreSQL if you expect
concurrent writes from multiple app instances or need higher write throughput.

```bash
createdb bugrail
createuser bugrail --pwprompt

# Add to /etc/bugrail.env:
BUGRAIL_DATABASE_URL=postgres://bugrail:password@localhost/bugrail?sslmode=disable
```

Restart the service and run `bugrail migrate` (or re-run `bugrail init` for a fresh install).

---

## Maintenance

**Upgrade the binary:**

```bash
install -o root -g root -m 755 bugrail-new /usr/local/bin/bugrail
systemctl restart bugrail
```

**Backup SQLite:**

```bash
# Safe online backup — flushes WAL before copying
sqlite3 /var/lib/bugrail/bugrail.sqlite3 ".backup /tmp/bugrail-$(date +%Y%m%d).sqlite3"
```

**View logs:**

```bash
journalctl -u bugrail -f
```

**Apply pending database migrations:**

```bash
sudo -u bugrail env $(cat /etc/bugrail.env | xargs) bugrail migrate
```
