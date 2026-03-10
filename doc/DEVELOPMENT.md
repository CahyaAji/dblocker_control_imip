## Development Guide (Laptop)

This file is for day-to-day development on your laptop.

### 1) First-time setup (new machine / fresh clone)

The MQTT broker requires a password file that is **not** committed to git. You must generate it once after cloning on any new machine.

Use the credentials from your `.env` file (defaults shown below):

```bash
docker run --rm -v "$PWD/mosquitto/config:/mosquitto/config" eclipse-mosquitto:2 \
  mosquitto_passwd -b -c /mosquitto/config/passwordfile "DBL0KER" "YOUR_MQTT_PASSWORD"
```

For example, using the default dev password from `.env`:

```bash
docker run --rm -v "$PWD/mosquitto/config:/mosquitto/config" eclipse-mosquitto:2 \
  mosquitto_passwd -b -c /mosquitto/config/passwordfile "DBL0KER" "4;1Yf,)\`"
```

This creates `mosquitto/config/passwordfile`. Without it, Mosquitto will crash on startup and the app will fail to connect.

### 2) Start the stack

```bash
cd /home/aji-c/Files/scm/2026/dblocker_control_imip
docker compose up -d
```

Check status:

```bash
docker compose ps
```

### 3) Frontend hot reload (fast development)

Instead of rebuilding Docker on every frontend change, run the Vite dev server locally while keeping the backend in Docker.

**Terminal 1 — backend (Docker):**
```bash
docker compose up -d
```

**Terminal 2 — frontend (local, hot reload):**
```bash
cd frontend
npm install   # only needed first time
npm run dev
```

Open **http://localhost:5173** instead of `localhost:8080`.

The Vite dev server automatically proxies `/api` and `/events` requests to the backend container on port `8080`, so everything works as normal — with instant hot reload on every file save.

When you're done and want to test the final build inside Docker:
```bash
docker compose up -d --build app
```

### 4) Update Go app after code changes

When you edit Go files and want to update only `dblocker-app`:

```bash
docker compose up -d --build app
```

Watch app logs:

```bash
docker compose logs -f --tail=100 app
```

### 5) Remove orphan containers

Use this if you changed compose services and old containers are still around:

```bash
docker compose up -d --build app --remove-orphans
```

### 6) Full refresh (if things are weird)

```bash
docker compose down
docker compose up -d --build
```

### 7) Beginner notes (important)

- Use service names inside Docker network (`mosquitto`, `postgres`), not `localhost`.
- Use `localhost` only from your laptop host to reach published ports.
- If app cannot connect to MQTT/DB, check logs first:

```bash
docker compose logs --tail=100 app mosquitto postgres
```

- If you changed `DB_PASSWORD` (or DB user/name), remember Postgres keeps old credentials inside the persisted `pgdata` volume from first initialization. Recreate DB volume for local dev:

```bash
docker compose down -v
docker compose up -d --build
```

- If build keeps old cache unexpectedly:

```bash
docker compose build --no-cache app
docker compose up -d app
```

- Clean unused images/cache occasionally:

```bash
docker image prune -f
docker builder prune -f
```

### 8) Quick command set

```bash
# update app only
docker compose up -d --build app

# update app + remove orphans
docker compose up -d --build app --remove-orphans

# check running containers
docker compose ps

# tail app logs
docker compose logs -f --tail=100 app
```

```bash
# update app and clean up
sudo docker compose up -d --build app && sudo docker image prune -f
```
