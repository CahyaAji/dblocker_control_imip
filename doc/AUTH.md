# Authentication Guide

## Token Duration

The JWT token is valid for **7 days** from the time of login. After that, the user needs to log in again. The token is stored in `localStorage` on the browser, so the user stays logged in across page refreshes.

---

## Default Admin

On first startup (when no users exist), the server auto-creates an admin user:

- **Username:** `admin` (override with `ADMIN_USERNAME` env var)
- **Password:** `admin` (override with `ADMIN_PASSWORD` env var)

> **Change the default password immediately after first login.**

---

## 1. Login as Admin

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}'
```

Response:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": 1,
    "username": "admin",
    "is_admin": true
  }
}
```

Save the token for subsequent requests:

```bash
TOKEN="eyJhbGciOiJIUzI1NiIs..."
```

---

## 2. Register a New User (Admin Only)

```bash
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"username": "operator1", "password": "mypassword", "is_admin": false}'
```

Response:

```json
{
  "user": {
    "id": 2,
    "username": "operator1",
    "is_admin": false
  }
}
```

To create another admin user, set `"is_admin": true`.

---

## 3. Login as the New User

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "operator1", "password": "mypassword"}'
```

Save the new token:

```bash
USER_TOKEN="eyJhbGciOiJIUzI1NiIs..."
```

---

## 4. Use That User to Turn On a DBlocker

First, get the dblocker list to find the ID:

```bash
curl http://localhost:8080/api/dblockers \
  -H "Authorization: Bearer $USER_TOKEN"
```

Then send a config update to turn on signals (e.g. GPS + Ctrl on all 6 sectors):

```bash
curl -X PUT http://localhost:8080/api/dblockers/config \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{
    "id": 1,
    "config": [
      {"signal_gps": true, "signal_ctrl": true},
      {"signal_gps": true, "signal_ctrl": true},
      {"signal_gps": true, "signal_ctrl": true},
      {"signal_gps": true, "signal_ctrl": true},
      {"signal_gps": true, "signal_ctrl": true},
      {"signal_gps": true, "signal_ctrl": true}
    ]
  }'
```

To turn off all sectors:

```bash
curl http://localhost:8080/api/dblockers/config/off/1 \
  -H "Authorization: Bearer $USER_TOKEN"
```

---

## 5. Change Admin Password

There is no dedicated "change password" endpoint. To change the admin password:

1. Login as admin and get the token.
2. Delete the old admin user and create a new one, **or** change it via the environment variables and recreate the database.

**Recommended approach** — delete and recreate:

```bash
# Login as admin
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

# Create a new admin with the desired password
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"username": "admin2", "password": "new_secure_password", "is_admin": true}'

# Login as the new admin
TOKEN2=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin2", "password": "new_secure_password"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

# Delete the old admin (ID 1)
curl -X DELETE http://localhost:8080/api/users/1 \
  -H "Authorization: Bearer $TOKEN2"
```

**Or set it before first startup** via `.env`:

```env
ADMIN_USERNAME=admin
ADMIN_PASSWORD=my_secure_password
```

Then `docker compose up -d`.

---

## 6. List All Users (Admin Only)

```bash
curl http://localhost:8080/api/users \
  -H "Authorization: Bearer $TOKEN"
```

---

## 7. Delete a User (Admin Only)

```bash
curl -X DELETE http://localhost:8080/api/users/2 \
  -H "Authorization: Bearer $TOKEN"
```

> An admin cannot delete themselves.

---

## Environment Variables

| Variable         | Default    | Description                         |
|------------------|------------|-------------------------------------|
| `JWT_SECRET`     | (random)   | Secret key for signing JWT tokens. Set a fixed value in production so tokens survive server restarts. |
| `ADMIN_USERNAME` | `admin`    | Default admin username (only used on first startup when no users exist). |
| `ADMIN_PASSWORD` | `admin`    | Default admin password (only used on first startup when no users exist). |
| `API_KEY`        | (disabled) | Static API key for service-to-service auth. Never expires. Set a long random string. |

---

## API Key (for `dblocker-assist-app`)

The `dblocker-assist-app` container uses an API key to call the backend API. The API key never expires and requires no login flow — it's designed for service-to-service communication between Docker containers.

### How It Works

```
┌────────────────────┐   X-API-Key header   ┌─────────────────┐
│ dblocker-assist-app │ ──────────────────► │ dblocker-app     │
│ (scheduler/sensors) │   via Docker network │ (backend API)   │
└────────────────────┘                       └─────────────────┘
```

Both containers share the same `API_KEY` env var from `.env`. The assist container calls the backend at `http://dblocker-app:8080` (Docker's internal network — never exposed to the internet).

### Step 1: Generate the Key

```bash
# Generate and append to .env
echo "API_KEY=$(openssl rand -hex 32)" >> .env
```

### Step 2: Verify `.env`

Your `.env` file should contain:

```env
DB_USER=scm
DB_PASSWORD=mysecretpassword
DB_NAME=dblocker-db
JWT_SECRET=some_fixed_secret_here
API_KEY=a1b2c3d4e5f6...your_64_char_hex...
ADMIN_PASSWORD=my_secure_password
```

### Step 3: Start/Restart

```bash
docker compose down && docker compose up -d
```

Both `dblocker-app` and `dblocker-assist-app` will receive the same `API_KEY`. The backend validates it, and the assist container sends it.

### Step 4: Verify It Works

From inside the assist container:

```bash
docker exec dblocker-assist-app wget -qO- \
  --header="X-API-Key: $(grep API_KEY .env | cut -d= -f2)" \
  http://dblocker-app:8080/api/dblockers
```

Or test from the host machine:

```bash
source .env
curl http://localhost:8080/api/dblockers \
  -H "X-API-Key: $API_KEY"
```

### Usage in Go (`cmd/assist/main.go`)

The assist container reads `API_KEY` and `BACKEND_URL` from its environment:

```go
apiKey := os.Getenv("API_KEY")        // shared secret
baseURL := os.Getenv("BACKEND_URL")   // http://dblocker-app:8080

// Turn off all sectors for dblocker ID 1
req, _ := http.NewRequest("GET", baseURL+"/api/dblockers/config/off/1", nil)
req.Header.Set("X-API-Key", apiKey)
resp, err := http.DefaultClient.Do(req)

// Turn on specific config
body := `{"id":1,"config":[
  {"signal_gps":true,"signal_ctrl":true},
  {"signal_gps":true,"signal_ctrl":true},
  {"signal_gps":true,"signal_ctrl":true},
  {"signal_gps":true,"signal_ctrl":true},
  {"signal_gps":true,"signal_ctrl":true},
  {"signal_gps":true,"signal_ctrl":true}
]}`
req, _ = http.NewRequest("PUT", baseURL+"/api/dblockers/config", strings.NewReader(body))
req.Header.Set("Content-Type", "application/json")
req.Header.Set("X-API-Key", apiKey)
resp, err = http.DefaultClient.Do(req)
```

### Docker Compose Wiring

The key is set once in `.env` and shared by both containers:

```yaml
# docker-compose.yml
services:
  app:
    container_name: dblocker-app
    environment:
      API_KEY: ${API_KEY:-}      # validates incoming keys
      ...

  assist:
    container_name: dblocker-assist-app
    environment:
      API_KEY: ${API_KEY:-}      # sends key with requests
      BACKEND_URL: http://dblocker-app:8080
```

### Rotating the Key

```bash
# Generate a new key
NEW_KEY=$(openssl rand -hex 32)

# Update .env (replace old key)
sed -i "s/^API_KEY=.*/API_KEY=$NEW_KEY/" .env

# Restart both containers to pick up the new key
docker compose down && docker compose up -d
```

The old key is immediately invalid after restart.

### Security Notes

1. **Docker network only** — Requests travel through Docker's internal bridge network (`http://dblocker-app:8080`), never over the public internet.
2. **Environment variable** — The key is never hardcoded in source code. It lives in `.env` (which should be in `.gitignore`).
3. **Non-admin** — API key requests are treated as a non-admin `_service` user. They can control dblockers but cannot create/delete users.
4. **Easy to rotate** — Change the `API_KEY` in `.env` and restart. One command.
5. **No expiration** — Unlike JWT tokens (7-day expiry), the API key works indefinitely until rotated. Perfect for long-running automated services.

> **Do NOT** use the API key from a browser or over the public internet. Browsers should use the JWT login flow.
