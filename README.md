# Strava Activity Analyzer (Go)

A web application that authenticates users via Strava, fetches their activity history, and presents read-only interactive analytics.

## Status
🚧 **Under Construction** - currently in development (Completed Phase 2: Authentication).

## Getting Started

### Prerequisites
*   Go 1.25.4 or higher.

### Configuration

1. **Copy the example environment file:**
   ```bash
   cp .env.example .env
   ```

2. **Get your Strava API credentials:**
   *   Visit [Strava API Settings](https://www.strava.com/settings/api)
   *   Create a new application or use an existing one
   *   Copy your `Client ID` and `Client Secret`

3. **Generate a secure SESSION_SECRET:**
   
   The `SESSION_SECRET` is required for secure session management. Generate a secure random string using one of these methods:
   
   **Option 1: Using OpenSSL (recommended)**
   ```bash
   openssl rand -hex 32
   ```
   
   **Option 2: Using Python**
   ```bash
   python3 -c "import secrets; print(secrets.token_hex(32))"
   ```
   
   **Option 3: Using Node.js**
   ```bash
   node -e "console.log(require('crypto').randomBytes(32).toString('hex'))"
   ```
   
   Copy the generated value and add it to your `.env` file:
   ```bash
   SESSION_SECRET=your-generated-secret-here
   ```

4. **Update your `.env` file:**
   ```bash
   STRAVA_CLIENT_ID=your_client_id_here
   STRAVA_CLIENT_SECRET=your_client_secret_here
   SESSION_SECRET=your-generated-secret-here
   PORT=8080
   ```

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
