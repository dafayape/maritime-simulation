<div align="center">

# 🚢 Maritime Ship Node — Mobile Node Client (Flutter)

**Software-in-the-Loop Edge Node Client with Offline Store-and-Forward & LoRa Data Link Engine**

A companion mobile terminal representing maritime vessel nodes within the *Maritime LoRa Mesh Network Simulator*. This application emulates hardware radio transceivers (ESP32/SX1276) in a software-in-the-loop setting: dynamically compiling user payloads into **pure binary serialization (MessagePack)**, managing transactional **offline Store-and-Forward** queues via SQLite, executing a deterministic **ACK & Auto-Retry state machine**, and orchestrating multi-hop mesh relays over WebSockets connected to the Go Virtual Ether.

[![ISO 25010](https://img.shields.io/badge/ISO%2FIEC-25010%20Compliant-success?logo=checkmarx&logoColor=white)](#iso-standards-compliance)
[![ISO 27001](https://img.shields.io/badge/ISO%2FIEC-27001%20Hardened-blue?logo=auth0&logoColor=white)](SECURITY.md)
[![ISO 12207](https://img.shields.io/badge/ISO%2FIEC-12207%20Lifecycle-orange)](#iso-standards-compliance)
[![Flutter](https://img.shields.io/badge/Flutter-3.x%20Riverpod-02569B?logo=flutter&logoColor=white)](pubspec.yaml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

</div>

---

## Multi-Tier Architecture Overview

| Component | Role | Branch / Location |
|---|---|---|
| `backend/` (Go) | Virtual Ether: radio physics, dynamic mesh routing, PostgreSQL & Redis | [`main`](../../tree/main) |
| `frontend/` (Next.js) | Real-time command dashboard & Leaflet geospatial monitoring | [`main`](../../tree/main) |
| **`mobile/` (Flutter)** | **Vessel node client: virtual LoRa packet originator & multi-hop relay** | **`mobile`** |

- **Version**: `1.0.0+1` (See [`CHANGELOG.md`](CHANGELOG.md))
- **Quality Gates**: `flutter analyze` 0 issues · **40/40** unit+widget tests passed · **3/3** live integration scenarios against Docker backend (`test_live/`).

---

## 1. Clean Architecture (Layered Design)

Strict adherence to industry Clean Architecture standards (**ISO/IEC 25010 & 12207**) across UI ↔ Domain ↔ Data layers with **Riverpod 2.6** for dependency injection and reactive state management:

```
lib/
├── main.dart / app.dart          # Entrypoint & MaterialApp configuration
├── core/
│   ├── config/                   # AppConfig (local preferences) & NodeSessionConfig
│   └── utils/backoff.dart        # Exponential backoff algorithm (1s→30s)
├── domain/                       # Pure business models free of IO/framework dependencies
│   ├── models/                   # EnvParams, SchemaField, RoutingInfo, TransmitFrame, etc.
│   ├── node_event.dart           # Sealed union of all WebSocket node events
│   └── node_link_state.dart      # Immutable state snapshot for UI consumers
├── data/
│   ├── services/                 # NodeSocket, RestClient, LocationService, QueueDatabase
│   └── repositories/             # NodeLinkRepository (Data Link Engine), ConfigRepository
├── di/providers.dart             # Unified Riverpod Dependency Injection graph
└── ui/
    ├── core/                     # AppTheme, custom vector AppLogo, shared UI widgets
    └── features/
        ├── splash/               # Animated splash screen
        ├── setup/                # Node parameters & connection preferences
        └── node/                 # Real-time HUD, dynamic forms, queue inspector, logs
```

---

## 2. Core Technical Capabilities

### A. LoRa Data Link State Machine
- **Automatic Retries & Exponential Backoff**: Retries unacknowledged transmissions up to 3 times before transitioning to `FAILED`, with backoff intervals dynamically clamped to 30s.
- **Store-and-Forward Disruption Tolerance (DTN)**: When isolated or out of radio coverage, outbound telemetry is queued into a transactional SQLite database and automatically flushed upon link restoration.
- **Pure Binary Serialization**: Packs dynamic field schemas into binary MessagePack payloads conforming to Spreading Factor byte constraints (SF7=222B down to SF12=51B).

### B. Mesh Relay Engine
- Seamlessly acts as an intermediate hop node: detects inbound packets, increments hop counters, appends routing traces, and forwards packets toward designated coastal Virtual Edges.

---

## 3. Automated Quality Verification

Run automated test suites locally:

```bash
# Static analysis
flutter analyze

# Unit & Widget test suite (40 tests)
flutter test

# Integration tests against live local backend (optional)
dart run test_live/backend_live_test.dart
```

---

## 🛡️ ISO Standards Compliance

| Standard | Scope | Implementation Details |
|---|---|---|
| **ISO/IEC 25010:2023**<br/>*(Software Product Quality)* | **Reliability & Fault Tolerance** | Offline SQLite queuing, transactional commit boundaries, zero unhandled link exceptions, and 100% test coverage. |
| **ISO/IEC 27001:2022**<br/>*(Information Security)* | **Integrity & Governance** | Commit signatures verified via **Ed25519 GPG Key** (`7573912AF4141E99`), zero credentials in code, and clear disclosure policy in [`SECURITY.md`](SECURITY.md). |
| **ISO/IEC 12207:2017**<br/>*(Software Life Cycle)* | **Lifecycle Management** | Layered Clean Architecture, Conventional Commits v1.0.0, and isolated branch release workflows. |

---

## License

Distributed under the MIT License. See [`LICENSE`](LICENSE) for details.

## Author

**Daffa Jaya Perkasa**  
Full-Stack | Mobile | AI | DevOps Engineer  
*GitHub: [@dafayape](https://github.com/dafayape)* • *Contact: [dafayape@gmail.com](mailto:dafayape@gmail.com)*
