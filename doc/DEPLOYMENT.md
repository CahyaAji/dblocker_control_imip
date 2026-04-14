## Production Deployment Guide

Step-by-step guide to deploy this project on a production server (VPS, bare metal, Raspberry Pi, etc.).

> **Quick command** (after all steps are done):
> ```
> docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d --build
> ```

---

### Step 1 — Install Docker on the server

```bash
# Install Docker (follow official docs for your distro, or use the convenience script)
curl -fsSL https://get.docker.com | sudo sh

# Enable and start Docker
sudo systemctl enable --now docker

# Verify
sudo docker --version
sudo docker compose version
```

### Step 2 — Open firewall ports

Open only the ports you need:

| Port   | Purpose                                  | Required? |
|--------|------------------------------------------|-----------|
| `22`   | SSH access                               | Yes       |
| `8080` | App API + dashboard                      | Yes       |
| `1883` | MQTT (only if external devices connect)  | Optional  |

Example with `ufw`:

```bash
sudo ufw allow 22/tcp
sudo ufw allow 8080/tcp
sudo ufw allow 1883/tcp   # only if external devices need direct MQTT
sudo ufw enable
```

### Step 3 — Get the project code on the server

**Option A — Clone from Git (recommended):**

```bash
sudo mkdir -p /opt
cd /opt
sudo git clone https://github.com/YOUR_ORG/YOUR_REPO.git dblocker_control_imip
sudo chown -R $USER:$USER /opt/dblocker_control_imip
cd /opt/dblocker_control_imip
```

For a private repo, use SSH:

```bash
git clone git@github.com:YOUR_ORG/YOUR_REPO.git dblocker_control_imip
```

**Option B — Copy from your laptop:**

```bash
# On your laptop
scp -r /path/to/dblocker_control_imip user@SERVER_IP:/opt/dblocker_control_imip

# Then on the server
cd /opt/dblocker_control_imip
```

### Step 4 — Create the `.env.prod` file

Copy the example file and edit it:

```bash
cp .env.prod.example .env.prod
nano .env.prod
```

Fill in **every** `CHANGE_ME_*` value. Here is what each variable does:

| Variable           | What to set                                                    |
|--------------------|----------------------------------------------------------------|
| `MQTT_PASSWORD`    | Strong password for the MQTT broker                            |
| `DB_PASSWORD`      | Strong password for PostgreSQL                                 |
| `JWT_SECRET`       | Random secret for JWT tokens. Generate with: `openssl rand -hex 32` |
| `API_KEY`          | Shared key between app and assist service. Generate with: `openssl rand -hex 32` |
| `ADMIN_PASSWORD`   | Password for the default admin user                            |
| `DRONE_DETECTORS`  | Comma-separated `host:port` pairs (e.g. `10.88.81.14:5555`)   |

> **Do not commit `.env.prod` to Git.** It is already in `.gitignore`.

### Step 5 — Generate the MQTT password file

The Mosquitto broker needs a password file that matches the `MQTT_USERNAME` and `MQTT_PASSWORD` in your `.env.prod`.

Replace `YOUR_MQTT_PASSWORD` below with the exact `MQTT_PASSWORD` value from `.env.prod`:

```bash
# Generate the password file
docker run --rm -v "$PWD/mosquitto/config:/mosquitto/config" eclipse-mosquitto:2 \
  mosquitto_passwd -b -c /mosquitto/config/passwordfile "DBL0KER" "YOUR_MQTT_PASSWORD"

# Set correct ownership and permissions
docker run --rm -v "$PWD/mosquitto/config:/mosquitto/config" alpine \
  sh -c "chown 1883:1883 /mosquitto/config/passwordfile && chmod 700 /mosquitto/config/passwordfile"
```

> The username `DBL0KER` must match `MQTT_USERNAME` in `.env.prod` (default is `DBL0KER`).

### Step 6 — Build and start all services

This project supports both ARM (Raspberry Pi) and AMD64. The Dockerfile detects the host architecture automatically.

```bash
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

This starts 4 containers:

| Container             | Role                              |
|-----------------------|-----------------------------------|
| `dblocker-app`        | Go backend + Svelte frontend      |
| `dblocker-assist-app` | Drone detector assist service     |
| `dblocker-mqtt`       | Mosquitto MQTT broker             |
| `dblocker-postgres`   | PostgreSQL database               |

### Step 7 — Verify everything is running

Check container status:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml ps
```

All containers should show `Up` status. Check logs:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml logs -f app mosquitto postgres
```

Open the dashboard in your browser:

```
http://SERVER_IP:8080/dashboard
```

Log in with the `ADMIN_USERNAME` and `ADMIN_PASSWORD` from `.env.prod`.

---

### Updating / Redeploying

```bash
cd /opt/dblocker_control_imip
git pull
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

### How clients connect to MQTT

| Context                          | Broker address              |
|----------------------------------|-----------------------------|
| Inside Docker (the Go app)       | `tcp://mosquitto:1883`      |
| Outside Docker (MCU / devices)   | `tcp://SERVER_IP:1883`      |

### Backup

Back up these regularly:

| Data              | Location                      |
|-------------------|-------------------------------|
| PostgreSQL        | Docker volume `pgdata`        |
| Mosquitto data    | `mosquitto/data/`             |
| Mosquitto config  | `mosquitto/config/`           |
| Environment file  | `.env.prod`                   |

---

### Troubleshooting

**1. App keeps restarting with `exec format error`**

Old image was built for a different CPU architecture. Rebuild from scratch:

```bash
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml down
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml build --no-cache --pull app
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d
```

Verify architecture:

```bash
docker image inspect dblocker_control_imip-app:latest --format '{{.Architecture}}/{{.Os}}'
```

**2. App log shows `mqtt connect error: not Authorized`**

`MQTT_PASSWORD` in `.env.prod` does not match the password file. Regenerate it:

```bash
docker run --rm -u 0 -v "$PWD/mosquitto/config:/mosquitto/config" eclipse-mosquitto:2 \
  mosquitto_passwd -b -c /mosquitto/config/passwordfile "DBL0KER" "YOUR_MQTT_PASSWORD"
docker run --rm -u 0 -v "$PWD/mosquitto/config:/mosquitto/config" alpine \
  sh -c "chown 1883:1883 /mosquitto/config/passwordfile && chmod 700 /mosquitto/config/passwordfile"
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml restart mosquitto app
```

**3. Cannot open `http://SERVER_IP:8080/dashboard`**

```bash
# Check if app is running
docker compose -f docker-compose.yml -f docker-compose.prod.yml ps
docker compose -f docker-compose.yml -f docker-compose.prod.yml logs --tail=100 app

# Test locally on the server
curl -i http://127.0.0.1:8080/dashboard
```

If local works but remote doesn't, it's a firewall issue:

```bash
sudo ufw allow 8080/tcp
```
