# Strava Activity Analyzer (Go)

A web application that authenticates users via Strava, fetches their activity history, and presents read-only interactive analytics.

## Status
🚧 **Under Construction** - currently in early development (Phase 1).

## Getting Started

### Prerequisites
*   Go 1.25.4 or higher.

### Quickstart

To start the application and automatically open it in your browser (at http://localhost:8080):

```bash
./quickstart.sh
```

## Features

### ✅ Implemented
*   **Project Foundation**: Initialized Go module and standard directory structure (`cmd/`, `internal/`, `web/`).

### 🚀 Planned
*   **Secure Authentication**: Strava OAuth2 connection with secure session management.
*   **Interactive Dashboard**:
    *   **Overview**: Activity counts and distribution.
    *   **Duration**: Moving time analytics.
    *   **Heatmaps**: Calendar-based training consistency visualization.
    *   **Running Stats**: PR tracking and distance histograms.
    *   **Trends**: Pace and distance progression over time.
*   **Data Handling**: Local timezone awareness and metric/imperial unit support.

## Project Structure
*   `cmd/`: Application entry points.
*   `internal/`: Core business logic and API clients.
*   `web/`: HTML templates and static assets.
*   `docs/`: Requirements, specifications, and development tasks.

## License
See [LICENSE](LICENSE) file.
