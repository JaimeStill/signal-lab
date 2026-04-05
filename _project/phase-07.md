# Phase 7 — Object Store

**Branch:** `phase-07-objstore`

## NATS Concepts

- **Object store buckets** — Named stores for arbitrarily large objects, built on JetStream
- **Chunked transfer** — Objects are automatically split into chunks for storage and reassembled on retrieval
- **Object metadata** — Name, description, size, checksum tracked per object
- **List/Get/Put/Delete** — Standard object operations

## Objective

Store sensor calibration profiles as JSON blobs in a NATS object store bucket. Sensor loads its calibration on startup. Dispatch can upload updated profiles that sensor can reload without restart.

## New Endpoints

```
Sensor:
  GET  /api/calibration            → current loaded calibration profile
  POST /api/calibration/reload     → re-download profile from object store and apply

Dispatch:
  POST /api/calibration/upload     → upload new calibration profile (multipart or JSON body)
  GET  /api/calibration/list       → list available calibration profiles with metadata
```

## Object Store Design

```
Bucket: calibration
  Objects: named calibration profiles (e.g., "sensor-default", "sensor-high-precision")
  Content: JSON calibration data
    {
      "name": "sensor-default",
      "readings": {
        "temp": {"min": -40, "max": 120, "precision": 0.5, "unit": "°F"},
        "humidity": {"min": 0, "max": 100, "precision": 1, "unit": "%"},
        "pressure": {"min": 800, "max": 1200, "precision": 0.1, "unit": "hPa"}
      },
      "sample_interval": "1s"
    }
```

## Files to Create/Modify

**`pkg/bus/objstore.go`** (new)
- Object store bucket creation/access helpers
- Put/Get/List/Delete wrappers

**`internal/sensor/telemetry.go`**
- Load calibration profile from object store on startup
- Apply calibration to reading generation (ranges, precision)
- Reload endpoint: fetch updated profile and apply without restart

**`internal/dispatch/api.go`**
- Calibration upload and list routes

## Verification

1. Start services → sensor loads default calibration from object store
2. `GET /api/calibration` on sensor → shows loaded profile
3. `POST /api/calibration/upload` on dispatch → upload modified profile
4. `GET /api/calibration/list` on dispatch → shows both profiles
5. `POST /api/calibration/reload` on sensor → applies new profile
6. Telemetry readings reflect new calibration parameters
