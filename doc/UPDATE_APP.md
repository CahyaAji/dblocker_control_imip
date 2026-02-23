## Update dblocker-app (Production)

Use this when your Go code changes and you want to redeploy only the app container.

### 1) Pull latest code

```bash
cd /opt/dblocker_control_imip
git pull
```

### 2) Rebuild and restart app only

```bash
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d --build app
```

### 3) Remove orphan containers

```bash
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d --remove-orphans
```

### 4) Check service status and logs

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml ps
```

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml logs -f --tail=100 app
```

### Optional cleanup (safe)

```bash
docker image prune -f
docker builder prune -f
```
