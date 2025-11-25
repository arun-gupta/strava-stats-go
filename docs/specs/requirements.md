# Requirements Document

## 1. Introduction
The Strava Activity Analyzer is a web application designed to authenticate with a user’s Strava account and present interactive analytics and visualizations about their physical activities. It provides deep insights into training habits, including heatmaps, distribution charts, and trend analysis, without social features or data mutation. The system ensures user privacy by processing data in a read-only manner and calculating metrics dynamically based on user-defined time windows.

## 2. Requirements

### Authentication & Security

1. **Strava OAuth2 Connection**
    - **User Story**: As a user, I want to connect my Strava account securely so that I can view my personal analytics without sharing my password directly with the app.
    - **Acceptance Criteria**:
        - WHEN the user clicks "Connect with Strava", THEN the system SHALL redirect the browser to Strava’s OAuth2 authorization page with the appropriate scopes (read, activity:read_all).
        - WHEN the user authorizes access, THEN the system SHALL exchange the authorization code for an access token and refresh token via the server-side.
        - WHEN the authentication is successful, THEN the system SHALL redirect the user to their personal dashboard.

2. **Token Management**
    - **User Story**: As a user, I want my session to remain active seamlessly so that I don't have to log in repeatedly during analysis.
    - **Acceptance Criteria**:
        - WHEN an access token expires, THEN the system SHALL automatically use the refresh token to obtain a new access token without user intervention.
        - WHEN a user explicitly selects "Sign Out", THEN the system SHALL invalidate the local session and discard active tokens.
        - WHEN handling tokens, THEN the system SHALL store them securely server-side and never expose them to the client browser.

### Data Acquisition & Processing

3. **Activity Fetching**
    - **User Story**: As a user, I want the system to download my activity history so that the charts reflect my actual performance data.
    - **Acceptance Criteria**:
        - WHEN the user logs in, THEN the system SHALL fetch activities from the Strava API using pagination until the history is retrieved or limits are reached.
        - WHEN fetching data, THEN the system SHALL respect Strava's API rate limits and implement backoff strategies if limits are exceeded.
        - WHEN processing activities, THEN the system SHALL normalize timestamps to the user's local timezone for accurate daily and weekly aggregation.

### Dashboard & Visualization

4. **Dashboard Navigation**
    - **User Story**: As a user, I want intuitive navigation so that I can easily return to the main dashboard view and see key metrics at a glance.
    - **Acceptance Criteria**:
        - WHEN the user clicks on "Strava Stats" branding in the top-left header, THEN the system SHALL navigate to the dashboard home page.
        - WHEN viewing the dashboard, THEN the system SHALL display a summary section above the tabs showing: Date Range (with start and end dates), Total Activities count, and Total Moving Time.
        - WHEN viewing the dashboard, THEN the system SHALL display tabs in the following order: Overview, Duration, Heatmap, Trends, Running Stats.

5. **Activity Distribution Widgets**
    - **User Story**: As a user, I want to see a breakdown of my sport types by count and duration so that I can understand my training focus.
    - **Acceptance Criteria**:
        - WHEN the "Overview" tab loads, THEN the system SHALL display an Activity Count Distribution pie/donut chart showing the number of activities per type (Run, Ride, Swim, etc.).
        - WHEN the "Time Distribution" tab loads, THEN the system SHALL display a chart showing total moving time aggregated by activity type.
        - WHEN viewing distribution charts, THEN the system SHALL display data labels directly on the chart slices showing count values or time values for segments representing more than 5% of the total.

6. **Training Heatmaps**
    - **User Story**: As a user, I want to visualize my training consistency on a calendar grid so that I can spot streaks and gaps.
    - **Acceptance Criteria**:
        - WHEN the "Heatmap" tab loads, THEN the system SHALL provide a mode toggle to switch between "All Activities" and "Running Only" views.
        - WHEN "All Activities" mode is selected, THEN the system SHALL colorize days based on the total time spent on all activities per day.
        - WHEN "Running Only" mode is selected, THEN the system SHALL only display running data and colorize days based on distance covered.
        - WHEN viewing the heatmap, THEN the system SHALL display a legend within the Heatmap tab showing time-based intensity levels: "No Activity", "< 1h", "1-2h", "2h+" for All Activities mode, and distance-based levels for Running Only mode.
        - WHEN viewing the heatmap, THEN the system SHALL display the calendar in a horizontal layout (days as rows, weeks as columns) that utilizes available screen width for better visual presentation.
        - WHEN viewing the heatmap, THEN the system SHALL display workout statistics including: Workout Days (total days with activity), Missed Days (days without activity), Current Streak (consecutive active days ending today), Days Since Last (days since most recent activity), Longest Gap (longest period without activity), and Total Gap Days (sum of all gap days).
        - WHEN the user clicks "Show Gap Details", THEN the system SHALL display a list of all gap periods (consecutive days without activity) in the selected date range, showing the start date, end date, and duration of each gap.

7. **Running Statistics**
    - **User Story**: As a runner, I want to see specific aggregated metrics and personal records so that I can track my running progress.
    - **Acceptance Criteria**:
        - WHEN viewing the "Running Stats" tab, THEN the system SHALL display the running summary statistics first, followed by the distance distribution histogram.
        - WHEN rendering "Running Stats", THEN the system SHALL display total runs, count of runs over 10K, total distance, and average pace.
        - WHEN calculating Personal Records (PRs), THEN the system SHALL identify the fastest mile, fastest 10K, longest run, and most elevation gain from the fetched activities.
        - WHEN rendering the distance distribution, THEN the system SHALL display a histogram of run distances in 1-mile (or 1-km) bins.

8. **Trend Analysis**
    - **User Story**: As a user, I want to see how my mileage and pace have changed over time so that I can identify training trends.
    - **Acceptance Criteria**:
        - WHEN the "Trends" tab loads, THEN the system SHALL provide a mode toggle to switch between "All Activities" and "Running Only" views.
        - WHEN rendering trends, THEN the system SHALL display line charts of distance and pace over time aggregated by day, week, or month.
        - WHEN "Running Only" mode is selected, THEN the system SHALL filter to running activities and display average running pace over time, handling pace inversions (speed vs. time/distance) correctly.
        - WHEN displaying trend lines, THEN the system SHALL apply smoothing (e.g., moving average) to reduce noise in daily data.

### User Controls & Configuration

9. **Date Filtering**
    - **User Story**: As a user, I want to filter my dashboard by specific dates so that I can analyze a specific training block or year.
    - **Acceptance Criteria**:
        - WHEN the dashboard loads, THEN the system SHALL default to showing the last 7 days of activity data.
        - WHEN the user selects a preset (e.g., 7 days, 30 days, YTD, All Time), THEN the system SHALL recompute all dashboard widgets to reflect only activities within that range.
        - WHEN the user selects a custom range, THEN the system SHALL allow picking specific start and end dates.
        - WHEN a filter is applied, THEN the system SHALL persist the selection in the URL or session so that a page refresh maintains the context.

10. **Unit Preferences**
- **User Story**: As a user, I want to toggle between metric and imperial units so that the data is presented in a format I understand.
- **Acceptance Criteria**:
    - WHEN the dashboard loads, THEN the system SHALL default to imperial units (miles, min/mile).
    - WHEN the user toggles the unit switch, THEN the system SHALL convert all distance (miles/km) and pace (min/mile vs min/km) metrics immediately.
    - WHEN a unit preference is set, THEN the system SHALL persist the selection in the URL or session so that a page refresh maintains the preference.

### System Quality & User Experience

11. **Error Handling & Empty States**
    - **User Story**: As a user, I want clear feedback when no data is available or an error occurs so that I know how to proceed.
    - **Acceptance Criteria**:
        - WHEN a selected date range contains no activities, THEN the system SHALL display a friendly "No activities found" message instead of broken charts.
        - WHEN an API error or network failure occurs, THEN the system SHALL display a descriptive error message with a retry option.

12. **Performance**
    - **User Story**: As a user, I want the dashboard to load quickly so that I have a smooth experience.
    - **Acceptance Criteria**:
        - WHEN the dashboard loads with a typical dataset (<5,000 activities), THEN the initial render SHALL complete within 2 seconds.
        - WHEN changing filters, THEN the chart updates SHALL occur within 300ms if data is already cached.