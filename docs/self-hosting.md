# Self-hosting Bugrail

Single binary, SQLite, systemd. Comptez 15 minutes sur un serveur Debian/Ubuntu vierge.

---

## Ce dont vous avez besoin

- Un VPS Linux (512 MB RAM minimum — un Hetzner CX11 à 4 €/mois convient)
- Accès root ou sudo
- Un nom de domaine **ou** simplement l'IP du serveur (voir étape 4)

---

## 1. Installer le binaire

Compilez depuis votre machine (Go 1.23+ requis) et uploadez :

```bash
# Sur votre machine locale
GOOS=linux GOARCH=amd64 go build -o bugrail-linux ./cmd/bugrail
scp bugrail-linux root@VOTRE_IP:/usr/local/bin/bugrail
```

Ou compilez directement sur le serveur :

```bash
apt install -y golang git
git clone https://github.com/Methamorphe/bugrail.git
cd bugrail
go build -o /usr/local/bin/bugrail ./cmd/bugrail
```

---

## 2. Préparer le serveur

```bash
useradd --system --no-create-home --shell /usr/sbin/nologin bugrail
mkdir -p /var/lib/bugrail
chown bugrail:bugrail /var/lib/bugrail
```

---

## 3. Créer le fichier de configuration

Remplacez `https://bugrail.example.com` par votre URL publique (voir étape 4 pour la choisir).

```bash
cat > /etc/bugrail.env << 'EOF'
BUGRAIL_DATA_DIR=/var/lib/bugrail
BUGRAIL_LISTEN_ADDR=127.0.0.1:8080
BUGRAIL_BASE_URL=https://bugrail.example.com
BUGRAIL_RATE_LIMIT_PER_PROJECT=1000
EOF
chmod 600 /etc/bugrail.env
```

`BUGRAIL_BASE_URL` est l'URL que verront vos SDKs. Elle apparaîtra dans le DSN affiché par `bugrail init`.

---

## 4. Choisir votre URL publique

**Option A — domaine ou sous-domaine (recommandé pour la production)**

Créez un enregistrement A `bugrail.example.com → VOTRE_IP` chez votre registrar, puis :

- `BUGRAIL_BASE_URL=https://bugrail.example.com`

**Option B — IP seule sans domaine, HTTPS gratuit via sslip.io**

`sslip.io` résout automatiquement `1-2-3-4.sslip.io` vers l'IP `1.2.3.4`. Let's Encrypt peut alors émettre un certificat dessus — sans acheter de domaine.

Remplacez les points de votre IP par des tirets :

```
IP : 1.2.3.4  →  hostname : bugrail.1-2-3-4.sslip.io
```

- `BUGRAIL_BASE_URL=https://bugrail.1-2-3-4.sslip.io`

**Option C — HTTP direct sur l'IP (pour tester rapidement)**

Pas de nginx, pas de TLS, Bugrail écoute directement sur le port 8080.

Dans `/etc/bugrail.env` :

```
BUGRAIL_LISTEN_ADDR=0.0.0.0:8080
BUGRAIL_BASE_URL=http://VOTRE_IP:8080
```

Puis ouvrez le port : `ufw allow 8080/tcp && ufw allow 22/tcp && ufw enable`

Passez directement à l'étape 6, il n'y a pas de nginx à configurer.

---

## 5. Configurer nginx + HTTPS (options A et B uniquement)

```bash
apt install -y nginx certbot python3-certbot-nginx
```

```bash
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

Remplacez `bugrail.example.com` par votre hostname (option A ou B).

Firewall :

```bash
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable
```

---

## 6. Initialiser la base de données

À faire **une seule fois**. Cette commande crée la DB, l'admin, et le premier projet.

```bash
set -a && source /etc/bugrail.env && set +a
sudo -u bugrail -E bugrail init \
  --admin-email votre@email.com \
  --admin-password "VotreMotDePasse" \
  --org-name "Mon Org" \
  --project-name "Mon App"
```

La sortie affiche votre DSN prêt à l'emploi :

```
Admin: votre@email.com
Project: Mon App
DSN: https://abc123@bugrail.example.com/1
```

---

## 7. Démarrer le service

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

Ouvrez votre URL publique dans un navigateur, connectez-vous avec les identifiants créés à l'étape 6.

---

## 8. Configurer votre SDK

Copiez le DSN affiché à l'étape 6 dans votre application :

```js
// JavaScript / Node
Sentry.init({ dsn: "https://abc123@bugrail.example.com/1" });
```

```python
# Python
sentry_sdk.init(dsn="https://abc123@bugrail.example.com/1")
```

```php
// PHP
\Sentry\init(['dsn' => 'https://abc123@bugrail.example.com/1']);
```

```go
// Go
sentry.Init(sentry.ClientOptions{Dsn: "https://abc123@bugrail.example.com/1"})
```

---

## Réinitialiser complètement

```bash
systemctl stop bugrail
rm /var/lib/bugrail/bugrail.sqlite3
# Relancer l'étape 6
```

---

## Maintenance

**Mettre à jour le binaire :**

```bash
# Uploadez le nouveau binaire puis :
install -o root -g root -m 755 bugrail-new /usr/local/bin/bugrail
systemctl restart bugrail
```

**Sauvegarder la base :**

```bash
sqlite3 /var/lib/bugrail/bugrail.sqlite3 ".backup /root/bugrail-$(date +%Y%m%d).sqlite3"
```

**Voir les logs :**

```bash
journalctl -u bugrail -f
```
