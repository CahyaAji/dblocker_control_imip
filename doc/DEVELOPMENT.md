## Development Guide (Laptop)

This file is for day-to-day development on your laptop.

### 1) Start the stack

```bash
cd /home/aji-c/Files/scm/2026/dblocker_control_imip
docker compose up -d
```

Check status:

```bash
docker compose ps
```

### 2) Update Go app after code changes

When you edit Go files and want to update only `dblocker-app`:

```bash
docker compose up -d --build app
```

Watch app logs:

```bash
docker compose logs -f --tail=100 app
```

### 3) Remove orphan containers

Use this if you changed compose services and old containers are still around:

```bash
docker compose up -d --build app --remove-orphans
```

### 4) Full refresh (if things are weird)

```bash
docker compose down
docker compose up -d --build
```

### 5) Beginner notes (important)

- Use service names inside Docker network (`mosquitto`, `postgres`), not `localhost`.
- Use `localhost` only from your laptop host to reach published ports.
- If app cannot connect to MQTT/DB, check logs first:

```bash
docker compose logs --tail=100 app mosquitto postgres
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

### 6) Quick command set

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
