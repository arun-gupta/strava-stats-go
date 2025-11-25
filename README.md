# Strava Activity Analyzer (Go)

A web application that authenticates users via Strava, fetches their activity history, and presents read-only interactive analytics.

## Status
🚧 **Under Construction** - currently in development (Completed Phase 2: Authentication).

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
*   **Project Foundation**: Basic HTTP server, standard directory structure (`cmd/`, `internal/`, `web/`), and configuration management (`.env`).
*   **Secure Authentication**:
    *   Full OAuth2 flow with Strava (Redirect & Callback).
    *   Secure session management using cookies.
    *   Automatic token refresh logic to maintain active sessions.
*   **Basic Dashboard**:
    *   Welcome page with "Connect with Strava" button.
    *   Authenticated state display ("Logged in as [Name]").

### 🚀 Planned
*   **Activity Integration**: Fetching activities from Strava API.
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
