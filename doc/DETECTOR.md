# Drone Detector Setup

## Overview

Drone detectors are devices that detect nearby drones via RF scanning. When a drone is detected, the system automatically activates the nearest blocker sectors pointing toward the drone.

## Adding Detectors

### Via API

```
POST /api/detectors
Authorization: Bearer <token>
Content-Type: application/json

{
    "name": "Detector Runway 13",
    "latitude": -2.793000,
    "longitude": 122.131000,
    "host": "10.88.81.100",
    "port": 5555
}
```

Fields:
- `name` — Display name (shown on map as a cyan dot)
- `latitude` / `longitude` — GPS coordinates of the detector
- `host` — IP address of the detector device (TCP)
- `port` — TCP port of the detector device (default: 5555)

### Via the assist service

The assist service (`dblocker-assist`) connects to detectors configured via the `DRONE_DETECTORS` environment variable:

```env
DRONE_DETECTORS=10.88.81.100:5555,10.88.81.101:5555
```

This establishes a persistent TCP connection to each detector and parses the binary protocol for heartbeat (type 1) and drone target (type 56) frames.

## How Auto-Activation Works

When a detector identifies a drone target:

1. The event is saved to the database (`POST /api/drone-events`)
2. All blockers are fetched from the backend
3. For each blocker, the **bearing** from the blocker to the drone is calculated using the haversine formula
4. The bearing is adjusted by the blocker's `angle_start` offset
5. The adjusted bearing maps to a 60° sector (sector 0 = 0°–60°, sector 1 = 60°–120°, etc.)
6. That sector's GPS and Ctrl signals are turned **ON** (preserving other already-active sectors)

### Example

- Blocker at (-2.80, 122.13) with `angle_start = 0`
- Drone detected at (-2.79, 122.14)
- Bearing ≈ 40° → Sector 0 activated (both GPS + Ctrl ON)

## Viewing Detections

- **Map**: Detectors appear as **cyan dots** (blockers are red)
- **Detections page**: Navigate to `/detections` or click the radar icon in the dashboard header — shows a live feed of detected drones with confidence, position, heading, speed and frequency
- **Action logs**: Auto-activations are logged with username `auto[detector-N]` and action `auto_drone_response`

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/detectors` | Create a detector |
| GET | `/api/detectors` | List all detectors |
| PUT | `/api/detectors/:id` | Update a detector |
| DELETE | `/api/detectors/:id` | Delete a detector |
| GET | `/api/drone-events?limit=N` | List recent drone events |
| GET | `/api/drone-events?detector_id=N` | Events by detector |
| POST | `/api/drone-events` | Create a drone event (used by assist service) |
