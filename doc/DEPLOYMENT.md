## Production Deployment Guide

This guide is for deploying this project from your laptop to a real production computer (server/VPS).

### 1) Prepare the production server

Install Docker and Docker Compose plugin.

```bash
sudo systemctl enable --now docker
sudo docker --version
sudo docker compose version
```

Open firewall ports:
- `22` for SSH
- `8080` for app API (or use reverse proxy later)
- `1883` only if external devices need direct MQTT access

### 2) Get project code on server

You can use either method below.

#### Option A: Clone from GitHub (recommended)

On server:

```bash
sudo mkdir -p /opt
cd /opt
sudo git clone https://github.com/YOUR_ORG/YOUR_REPO.git dblocker_control_imip
sudo chown -R $USER:$USER /opt/dblocker_control_imip
cd /opt/dblocker_control_imip
```

If the repository is private, use SSH URL instead:

```bash
git clone git@github.com:YOUR_ORG/YOUR_REPO.git dblocker_control_imip
```

#### Option B: Copy from laptop using scp

From your laptop:

```bash
scp -r /path/to/dblocker_control_imip user@SERVER_IP:/opt/dblocker_control_imip
```

On server:

```bash
cd /opt/dblocker_control_imip
```

### 3) Create production environment file

```bash
cp .env.prod.example .env.prod
nano .env.prod
```

Set strong production values in `.env.prod`, especially:
- `MQTT_PASSWORD`
- `DB_PASSWORD`

Do not commit `.env.prod` into Git.

### 4) Generate MQTT password file on server

Run this on the production server (uses your intended username/password):

```bash
docker run --rm -v "$PWD/mosquitto/config:/mosquitto/config" eclipse-mosquitto:2 \
  mosquitto_passwd -b -c /mosquitto/config/passwordfile "DBL0KER" "YOUR_MQTT_PASSWORD_FROM_ENV"
```

Set secure permissions:

```bash
docker run --rm -v "$PWD/mosquitto/config:/mosquitto/config" alpine \
  sh -c "chown 1883:1883 /mosquitto/config/passwordfile && chmod 700 /mosquitto/config/passwordfile"
```

### 5) Start in production mode

```bash
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

Check status:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml ps
```

Check logs:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml logs -f app mosquitto postgres
```

### 6) How clients connect to MQTT

- Inside Docker (your Go app): `tcp://mosquitto:1883`
- Outside Docker (other devices): `tcp://SERVER_IP:1883`

Both are correct for different network contexts.

### 7) Update / redeploy later

```bash
cd /opt/dblocker_control_imip
git pull
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

If repository is private over HTTPS, use a GitHub Personal Access Token when prompted,
or configure SSH keys once and use the SSH remote URL.

### 8) Backup important data

Persisted data locations:
- Postgres volume: Docker volume `pgdata`
- Mosquitto data: `mosquitto/data`
- Mosquitto config + password file: `mosquitto/config`

Back up these regularly.
