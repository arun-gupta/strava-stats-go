# Strava Activity Analyzer (Go)

A web application that authenticates users via Strava, fetches their activity history, and presents read-only interactive analytics.

## Status
🚧 **Under Construction** - currently in early development (Phase 1).

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

## Getting Started

### Prerequisites
*   Go 1.25.4 or higher.

### Installation
```bash
# Clone the repository
git clone https://github.com/arungupta/strava-stats-go.git

# Navigate to project root
cd strava-stats-go

# Download dependencies
go mod download
```

## Project Structure
*   `cmd/`: Application entry points.
*   `internal/`: Core business logic and API clients.
*   `web/`: HTML templates and static assets.
*   `docs/`: Requirements, specifications, and development tasks.

## License
See [LICENSE](LICENSE) file.
