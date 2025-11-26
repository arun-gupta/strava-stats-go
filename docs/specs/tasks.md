# Development Tasks: Strava Activity Analyzer

Revised to prioritize iterative delivery of visible features.

## Phase 1: Project Foundation & "Hello World"
- [x] 1.1: Initialize Go module (`go mod init`).
- [x] 1.2: Create directory structure (`cmd/`, `internal/`, `web/`).
- [x] 1.3: Implement basic HTTP server.
- [x] 1.4: Create a simple "Welcome" HTML template.
- [x] 1.5: Verify app runs and serves the welcome page in browser.

## Phase 2: Authentication & User Identity
- [x] 2.1: Set up configuration management for Strava Client ID/Secret (e.g., `.env`).
- [x] 2.2: Implement OAuth2 flow: Redirect to Strava authorization page.
- [x] 2.3: Implement OAuth2 flow: Handle callback and exchange codes for tokens.
- [x] 2.4: Implement session management (store tokens).
- [x] 2.5: Implement token refresh logic to maintain active sessions.
- [x] 2.6: Update dashboard to display "Logged in as [User Name]" after auth.
- [x] 2.7: Add a top-level Strava colored banner with title on the left and octocat logo pointing to GitHub repo, athlete photo, and a Logout button on the right.

## Phase 3: Connectivity & Activity List
- [x] 3.1: Create HTML templates for the dashboard structure (Tabbed Layout with conditional visibility: Overview, Duration, Heatmap, Running Stats, Trends). Add Coming Soon to the sections that are to be implemented.
- [x] 3.2: Show a summary as three horizontal cards: start date to end date, total number of activities, total moving time.
- [x] 3.3: Build Strava API client to fetch activities (`/athlete/activities`).
- [x] 3.4: Implement data normalization (parse `start_date_local` for date alignment, unit standardization). Use Last 7 Days as default. If a workout occurred on a day, independent of timezone, it must be included.
- [x] 3.5: Wire up frontend to fetch data from Go backend and fix up summary cards.
- [x] 3.6: Render a simple list of activities (default last 7 days based on local date) on the dashboard, in the overview tab.
- [x] 3.7: Implement HTTP error handling (429 Rate Limits, 401 Unauthorized/Refresh, 5xx Server Errors).

## Phase 4: Core Visualizations (Distributions)
- [ ] 4.1: Integrate JavaScript charting library (e.g., Chart.js).
- [ ] 4.2: Calculate activity counts by sport type.
- [ ] 4.3: Render Activity Counts Distribution (Pie/Donut chart) on the Overview tab.
- [ ] 4.4: Calculate moving time by sport type.
- [ ] 4.5: Render Moving Time Distribution (Pie/Donut chart) on the Duration tab.

## Phase 5: Advanced Analytics (Heatmaps & Trends)
- [ ] 5.1: Implement pagination handling to retrieve full activity history.
- [ ] 5.2: Implement grid generation for Activity Heatmap (frequency/intensity).
- [ ] 5.3: Render Heatmaps (Calendar grid).
- [ ] 5.4: Implement logic for calculating running averages (Pace/Distance).
- [ ] 5.5: Render Trends (Line charts).
- [ ] 5.6: Implement algorithms to find PRs & generate distance histograms.

## Phase 6: Refinement & Interactivity
- [ ] 6.1: Implement Date Range Picker (ensure filtering uses local activity dates).
- [ ] 6.2: Implement Unit Toggle (Metric/Imperial).
- [ ] 6.3: Implement error handling (empty states hiding tabs, API failures).
- [ ] 6.4: Optimize initial load time (concurrent fetching).
- [ ] 6.5: Ensure dashboard responsiveness.
- [ ] 6.6: Verify client-side unit conversion logic.
