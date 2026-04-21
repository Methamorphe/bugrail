# Self-hosting Bugrail

This guide walks you through deploying Bugrail on a Linux VPS so you have a real public DSN
to point your SDKs at. The result: a single binary behind nginx, running as a systemd service,
with HTTPS via Certbot.

Estimated time: 20 minutes on a fresh Debian/Ubuntu server.

---

## Requirements

- A VPS with at least 512 MB RAM and 2 GB disk (a 5 €/month Hetzner CX11 or equivalent is fine)
- A domain or subdomain pointing to that server (e.g. `bugrail.example.com`)
- Go 1.23+ **or** a pre-built binary from the releases page
- Root or sudo access on the server

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

# Optional: use PostgreSQL instead of SQLite (see section 7).
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

The command prints a DSN. **Ignore the hostname for now** — it will show `localhost:8080`
because the binary doesn't know your public URL yet. You will reconstruct the real DSN in
section 6 after nginx is configured.

Example output:

```
Admin: you@example.com
Project: My App
DSN: http://<key>@localhost:8080/1
```

Note the `<key>` part — you will need it in section 6.

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
# Harden the process
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

## 6. Set up nginx as a reverse proxy with HTTPS

Install nginx and Certbot:

```bash
apt install -y nginx certbot python3-certbot-nginx
```

Create a site config:

```bash
cat > /etc/nginx/sites-available/bugrail << 'EOF'
server {
    listen 80;
    server_name bugrail.example.com;  # <-- replace with your domain

    # Certbot will rewrite this block to HTTPS automatically.
    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;

        # Sentry SDKs send large payloads (source maps, attachments).
        client_max_body_size 50m;
    }
}
EOF

ln -s /etc/nginx/sites-available/bugrail /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx
```

Obtain a TLS certificate:

```bash
certbot --nginx -d bugrail.example.com
```

Certbot will modify the nginx config to redirect HTTP to HTTPS and add the certificate.
Test renewal: `certbot renew --dry-run`.

---

## 7. Construct your real DSN

The DSN format is:

```
https://<key>@<your-domain>/<project-id>
```

From the `bugrail init` output:
- `<key>` — the long hex string shown after the `@` in the printed DSN
- `<your-domain>` — `bugrail.example.com` (or whatever you configured)
- `<project-id>` — the number at the end (usually `1` for the first project)

Example:

```
https://abc123def456@bugrail.example.com/1
```

Use this in your SDK config. You can also find the key in the Bugrail dashboard under the
project settings page once you log in.

---

## 8. Point your SDK at Bugrail

Replace the Sentry DSN in your application. No other change needed.

```js
// Node / JavaScript
Sentry.init({ dsn: "https://<key>@bugrail.example.com/1" });
```

```python
# Python
sentry_sdk.init(dsn="https://<key>@bugrail.example.com/1")
```

```php
// PHP
\Sentry\init(['dsn' => 'https://<key>@bugrail.example.com/1']);
```

```go
// Go
sentry.Init(sentry.ClientOptions{Dsn: "https://<key>@bugrail.example.com/1"})
```

---

## 9. Verify end-to-end

Trigger a test error in your app (or use the seed tool):

```bash
# From a machine that can reach bugrail.example.com
go run ./cmd/seed https://<key>@bugrail.example.com/1
```

Then open `https://bugrail.example.com` in a browser, log in, and check the dashboard.

---

## Optional: PostgreSQL instead of SQLite

SQLite is the right default for most deployments. Switch to PostgreSQL if you expect
concurrent writes from multiple app instances or need WAL-mode performance at scale.

```bash
# Create the database
createdb bugrail
createuser bugrail --pwprompt

# Add to /etc/bugrail.env
BUGRAIL_DATABASE_URL=postgres://bugrail:password@localhost/bugrail?sslmode=disable
```

Remove the `BUGRAIL_DATA_DIR` line if you no longer need SQLite. Restart the service and
re-run `bugrail init` (or `bugrail migrate` if re-initializing an existing deployment).

---

## Maintenance

**Upgrade the binary:**

```bash
# Build or download the new binary to a temp path, then atomically replace
install -o root -g root -m 755 bugrail-new /usr/local/bin/bugrail
systemctl restart bugrail
```

**Backup SQLite:**

```bash
# Safe online backup — SQLite's .backup command flushes WAL before copying
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

---

## Firewall

Only ports 80 and 443 need to be open to the internet. Bugrail's HTTP server listens on
`127.0.0.1:8080` and should never be exposed directly.

```bash
ufw allow 22/tcp   # SSH
ufw allow 80/tcp   # HTTP (redirects to HTTPS via nginx)
ufw allow 443/tcp  # HTTPS
ufw enable
```
