# Strava Activity Analyzer (Go)

A web application that authenticates users via Strava, fetches their activity history, and presents read-only interactive analytics.

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

#### Phase 1-2: Foundation & Authentication
*   **Project Foundation**: Basic HTTP server, standard directory structure (`cmd/`, `internal/`, `web/`), and configuration management (`.env`).
*   **Secure Authentication**:
    *   Full OAuth2 flow with Strava (Redirect & Callback).
    *   CSRF protection with cryptographically random state parameters.
    *   Secure session management using cookies with required `SESSION_SECRET`.
    *   Automatic token refresh logic to maintain active sessions.
*   **User Interface**:
    *   Welcome page with "Connect with Strava" button.
    *   Authenticated state display ("Logged in as [Name]").
    *   Strava-themed header with branding and logout functionality.

#### Phase 3: Connectivity & Activity List
*   **Strava API Integration**:
    *   Activity fetching with pagination support.
    *   Robust error handling (rate limits, unauthorized, server errors).
    *   Automatic token refresh and retry logic.
*   **Data Normalization**:
    *   Timezone-independent date alignment using `start_date_local`.
    *   Unit standardization (meters to km/miles, seconds to formatted duration).
    *   Default 7-day activity window.
*   **Dashboard Summary**:
    *   Date range display (start to end date).
    *   Total activities count.
    *   Total moving time.

#### Phase 4: Core Visualizations
*   **Overview Tab**:
    *   Activity counts distribution (pie/doughnut chart) by sport type.
*   **Duration Tab**:
    *   Moving time distribution (pie/doughnut chart) by sport type.

#### Phase 5: Advanced Analytics
*   **Heatmap Tab**:
    *   Calendar-based activity heatmap showing training consistency.
    *   Intensity levels based on moving time.
    *   Toggle between "All Activities" and "Running Only" views.
*   **Running Stats Tab**:
    *   Running summary statistics (Total Runs, 10K+ Runs, Total Distance, Average Pace).
    *   Personal Records (Fastest 10K, Longest Run).
    *   Distance distribution histogram (1-mile bins).
*   **Trends Tab**:
    *   Distance trend line chart over time.
    *   Pace trend line chart over time.
    *   Period toggle (Daily, Weekly, Monthly).
    *   Shows all activities (no filter toggle).

### 🚀 Planned (Phase 6)
*   **Date Range Picker**: Preset options (Last 7 Days, 30 days, 90 days, YTD) and custom date selection.
*   **Unit Toggle**: Switch between Metric and Imperial units.
*   **Enhanced Error Handling**: Empty states, improved API failure messages.
*   **Performance Optimization**: Concurrent data fetching, optimized initial load.
*   **Responsive Design**: Mobile-friendly dashboard layout.

## Project Structure
*   `cmd/`: Application entry points.
*   `internal/`: Core business logic and API clients.
*   `web/`: HTML templates and static assets.
*   `docs/`: Requirements, specifications, and development tasks.

## License
See [LICENSE](LICENSE) file.
