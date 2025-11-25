# Development Plan: Strava Activity Analyzer

## 1. Overview
The goal is to build a Go-based web application that authenticates users via Strava, fetches their activity history, and presents read-only interactive analytics. The system focuses on deep insights like heatmaps, trends, and running statistics, ensuring user privacy and transient data processing.

## 2. Main Steps & Phases

### Phase 1: Project Initialization & Configuration
- **Objective:** Set up the Go project structure and environment.
- Initialize Go module (`go mod init`).
- Create directory structure (`cmd/`, `internal/`, `web/`).
- Set up configuration management for Strava Client ID/Secret (e.g., `.env`).

### Phase 2: Authentication & Session Management
- **Objective:** Securely connect to Strava.
- Implement OAuth2 flow: Redirect to Strava, handle callback, exchange codes for tokens.
- Implement session management to store tokens server-side (in-memory or secure cookie).
- Implement token refresh logic to maintain active sessions.

### Phase 3: Strava API Client & Data Ingestion
- **Objective:** Reliable data fetching.
- Build a Strava API client to fetch activities (`/athlete/activities`).
- Implement pagination handling to retrieve full activity history.
- Implement rate-limiting handling (respect HTTP 429 headers).
- Implement data normalization (timezone adjustments, unit standardization).

### Phase 4: Core Analytics Engine
- **Objective:** Process raw data into meaningful metrics.
- **Overview:** Calculate activity counts, total moving time, and split by sport type. Show them in two separate pie charts.
- **Heatmap Logic:** Grid generation for calendar views (frequency/intensity), one for activity and another for running, measured by time spent.
- **Running Stats:** Algorithms to find PRs (fastest mile, 10k, etc.) and generate distance histograms.
- **Trends:** Logic for calculating running averages of pace and distance over time.

### Phase 5: Web Interface & Visualization
- **Objective:** User-facing dashboard.
- Create HTML templates for the dashboard structure with a tabbed layout (Overview, Duration, Heatmap, Running Stats, Trends) that is conditionally rendered based on data availability.
- Integrate a JavaScript charting library (e.g., Chart.js) for rendering:
  - Distributions (Pie/Donut charts).
  - Heatmaps (Calendar grid).
  - Trends (Line charts).
- Implement interactive controls: Date Range Picker, Unit Toggle (Metric/Imperial).
- Wire up frontend to fetch processed data from Go backend endpoints.

### Phase 6: Refinement & Quality
- **Objective:** Polish and Performance.
- Implement error handling (empty states hiding tabs, API failures).
- Optimize initial load time (concurrent fetching where possible).
- Ensure dashboard responsiveness and unit conversion logic on the client side.

## 3. Dependencies, Risks, and Considerations

### Dependencies
- **Go:** Backend language.
- **Strava API V3:** Primary data source.
- **Frontend Library:** Lightweight JS for charts (e.g., Chart.js, uPlot).

### Risks
- **API Rate Limiting:** Strava limits (100 requests/15 mins, 1000/day) are a major constraint. Efficient caching logic is mandatory.
- **Data Volume:** Users with thousands of activities may experience slow initial loads. Pagination must be handled efficiently.
- **Privacy:** Tokens must never leak to the client.

### Considerations
- **Timezones:** Activities must be visualized using their local start time (`start_date_local`) to accurately reflect the day they occurred, regardless of the viewer's current timezone.
- **Unit Conversion:** Backend should likely normalize to one standard (e.g., Metric), with frontend handling the display conversion based on user preference.
