## Notes

### 1. Start docker
```
sudo systemctl start docker
```

```
docker compose up -d
```
up: Tells Docker to read the YML file, set up the network, and start all the containers listed inside.

-d: Stands for "detached mode." This runs the containers in the background so you get your terminal prompt back immediately.

```
cmd/
  server/
    main.go                 # wire config, logger, DB, MQTT, services, routes

internal/
  model/
    device.go               # domain models/value objects
    reading.go
  service/
    bridge.go               # subscribes to MQTT topics, exposes channel/callback
    device.go               # business logic; depends on repository interfaces
    auth.go                 # (optional) auth rules
  infrastructure/
    mqtt/
      client.go             # Paho wrapper; implements BridgeSubscriber interface
    db/
      connection.go         # opens DB pool
      migrate.go            # migrations hook
      repo/
        device_repo.go      # implements service.DeviceRepository
    logger/
      logger.go             # zap/logrus setup
    cache/                  # (optional)
  handler/
    http/
      device_handler.go     # REST handlers; bind/validate; call services
      health_handler.go
      bridge_sse_handler.go # SSE endpoint reads from bridge service/broadcaster
    mqtt/                   # (optional) if you process inbound MQTT → services
    dto/
      device_request.go     # HTTP DTOs; keep transport-specific
      device_response.go
  route/
    http_routes.go          # register routes/middleware; uses handler/http
  bridge/                   # optional if you keep it separate
    broadcaster.go          # SSE fan-out; can be folded into handler/sse

pkg/                        # optional, reusable helpers (config, telemetry, etc.)
db/
  migrations/               # SQL migrations
```