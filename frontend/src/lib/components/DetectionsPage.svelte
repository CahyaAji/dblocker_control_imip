<script lang="ts">
    import { onMount } from "svelte";
    import {
        authStore,
        authFetch,
        verifyToken,
        logout,
    } from "../store/authStore";
    import { API_BASE } from "../utils/api";
    import LoginPage from "./LoginPage.svelte";

    interface DroneDetector {
        id: number;
        name: string;
        latitude: number;
        longitude: number;
        host: string;
        port: number;
        status: string;
        last_seen: string;
    }

    interface DroneEvent {
        id: number;
        detector_id: number;
        detector: string;
        unique_id: string;
        target_name: string;
        drone_lat: number;
        drone_lng: number;
        drone_alt: number;
        heading: number;
        distance: number;
        speed: number;
        frequency: number;
        confidence: number;
        remote_lat: number;
        remote_lng: number;
        created_at: string;
    }

    let detectors = $state<DroneDetector[]>([]);
    let events = $state<DroneEvent[]>([]);
    let loading = $state(false);
    let authorized = $state(false);
    let darkMode = $state(true);
    let autoRefresh = $state(true);
    let refreshTimer: ReturnType<typeof setInterval> | undefined;

    // Date filter – defaults to today
    let filterDate = $state(new Date().toISOString().slice(0, 10));

    function loadTheme() {
        try {
            const stored = localStorage.getItem("app-settings");
            if (stored) {
                const parsed = JSON.parse(stored);
                darkMode = parsed.theme === "dark";
            }
        } catch {
            /* */
        }
        applyTheme();
    }

    function applyTheme() {
        if (typeof document !== "undefined") {
            document.documentElement.classList.toggle("dark", darkMode);
        }
    }

    function toggleTheme() {
        darkMode = !darkMode;
        applyTheme();
        try {
            const stored = localStorage.getItem("app-settings");
            const settings = stored ? JSON.parse(stored) : {};
            settings.theme = darkMode ? "dark" : "light";
            localStorage.setItem("app-settings", JSON.stringify(settings));
        } catch {
            /* */
        }
    }

    onMount(() => {
        loadTheme();
        if ($authStore.token) {
            verifyToken().then((valid) => {
                if (valid) {
                    authorized = true;
                    fetchAll();
                    startAutoRefresh();
                }
            });
        }
        return () => stopAutoRefresh();
    });

    function startAutoRefresh() {
        stopAutoRefresh();
        if (autoRefresh) {
            refreshTimer = setInterval(fetchAll, 5000);
        }
    }

    function stopAutoRefresh() {
        if (refreshTimer) {
            clearInterval(refreshTimer);
            refreshTimer = undefined;
        }
    }

    $effect(() => {
        if (authorized) {
            if (autoRefresh) {
                startAutoRefresh();
            } else {
                stopAutoRefresh();
            }
        }
    });

    async function fetchAll() {
        await Promise.all([fetchDetectors(), fetchEvents()]);
    }

    async function fetchDetectors() {
        try {
            const res = await authFetch(`${API_BASE}/api/detectors`);
            if (res.ok) {
                const json = await res.json();
                detectors = json.data || [];
            }
        } catch {
            console.error("Failed to fetch detectors");
        }
    }

    async function fetchEvents() {
        loading = true;
        try {
            const res = await authFetch(
                `${API_BASE}/api/drone-events?from=${filterDate}&to=${filterDate}&limit=500`,
            );
            if (res.ok) {
                const json = await res.json();
                events = json.data || [];
            }
        } catch {
            console.error("Failed to fetch drone events");
        } finally {
            loading = false;
        }
    }

    function localTzLabel(): string {
        const offset = -new Date().getTimezoneOffset();
        const sign = offset >= 0 ? "+" : "-";
        const h = Math.floor(Math.abs(offset) / 60)
            .toString()
            .padStart(2, "0");
        const m = (Math.abs(offset) % 60).toString().padStart(2, "0");
        return `UTC${sign}${h}:${m}`;
    }

    function formatTime(ts: string): string {
        const d = new Date(ts);
        return d.toLocaleString("en-GB", {
            year: "numeric",
            month: "short",
            day: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
        });
    }

    function confidenceClass(c: number): string {
        if (c >= 80) return "high";
        if (c >= 50) return "medium";
        return "low";
    }

    function formatLastSeen(ts: string): string {
        if (!ts || ts === "0001-01-01T00:00:00Z") return "Never";
        const d = new Date(ts);
        const now = Date.now();
        const diff = Math.floor((now - d.getTime()) / 1000);
        if (diff < 60) return `${diff}s ago`;
        if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
        if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
        return d.toLocaleDateString("en-GB", {
            month: "short",
            day: "2-digit",
        });
    }
</script>

{#if !$authStore.token}
    <LoginPage />
{:else if !authorized}
    <div class="page">
        <div class="access-denied">
            <h2>Loading...</h2>
        </div>
    </div>
{:else}
    <div class="page">
        <header class="page-header">
            <div class="header-left">
                <a href="/dashboard" class="back-link">← Dashboard</a>
                <h1>Drone Detections</h1>
                <span class="event-count">{events.length} events</span>
            </div>
            <div class="header-right">
                <label class="auto-toggle">
                    <input type="checkbox" bind:checked={autoRefresh} />
                    <span>Auto-refresh</span>
                </label>
                <button
                    class="btn-theme"
                    onclick={toggleTheme}
                    title="Toggle theme"
                >
                    {#if darkMode}☀{:else}🌙{/if}
                </button>
                <span class="user-label">{$authStore.user?.username}</span>
                <button class="btn-logout" onclick={logout}>Logout</button>
            </div>
        </header>

        {#if detectors.length > 0}
            <div class="detector-status-bar">
                {#each detectors as det (det.id)}
                    <div
                        class="detector-chip"
                        class:online={det.status === "online"}
                    >
                        <span
                            class="status-dot"
                            class:online={det.status === "online"}
                        ></span>
                        <span class="det-name">{det.name}</span>
                        <span class="det-addr">{det.host}:{det.port}</span>
                        <span class="det-seen"
                            >{formatLastSeen(det.last_seen)}</span
                        >
                    </div>
                {/each}
            </div>
        {/if}

        <div class="filter-bar">
            <label class="filter-label">
                Date
                <input
                    type="date"
                    class="filter-input"
                    bind:value={filterDate}
                    onchange={() => fetchEvents()}
                />
            </label>
        </div>

        {#if loading && events.length === 0}
            <div class="empty">Loading...</div>
        {:else if events.length === 0}
            <div class="empty">
                <div class="empty-icon">📡</div>
                <div>No drone detections yet</div>
                <div class="empty-sub">
                    Events will appear here when a drone detector identifies a
                    target
                </div>
            </div>
        {:else}
            <div class="events-list">
                <table class="events-table">
                    <thead>
                        <tr>
                            <th>Time ({localTzLabel()})</th>
                            <th>Target</th>
                            <th>Detector</th>
                            <th>Confidence</th>
                            <th>Position</th>
                            <th>Alt</th>
                            <th>Heading</th>
                            <th>Dist</th>
                            <th>Speed</th>
                            <th>Freq</th>
                        </tr>
                    </thead>
                    <tbody>
                        {#each events as ev (ev.id)}
                            <tr>
                                <td class="col-time"
                                    >{formatTime(ev.created_at)}</td
                                >
                                <td class="col-target"
                                    >{ev.target_name ||
                                        ev.unique_id ||
                                        "Unknown"}</td
                                >
                                <td
                                    ><span class="detector-label"
                                        >{ev.detector}</span
                                    ></td
                                >
                                <td
                                    ><span
                                        class="confidence-badge {confidenceClass(
                                            ev.confidence,
                                        )}">{ev.confidence}%</span
                                    ></td
                                >
                                <td class="col-mono"
                                    >{ev.drone_lat.toFixed(5)}, {ev.drone_lng.toFixed(
                                        5,
                                    )}</td
                                >
                                <td class="col-mono">{ev.drone_alt}m</td>
                                <td class="col-mono">{ev.heading}°</td>
                                <td class="col-mono">{ev.distance}m</td>
                                <td class="col-mono">{ev.speed.toFixed(1)}</td>
                                <td class="col-mono"
                                    >{ev.frequency.toFixed(0)}</td
                                >
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
        {/if}
    </div>
{/if}

<style>
    .page {
        max-width: 1100px;
        margin: 0 auto;
        padding: 24px 16px;
        min-height: 100vh;
    }

    .access-denied {
        text-align: center;
        padding: 60px 20px;
        color: var(--text-secondary);
    }

    .page-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 20px;
        flex-wrap: wrap;
        gap: 12px;
    }

    .header-left {
        display: flex;
        align-items: center;
        gap: 16px;
    }

    .header-right {
        display: flex;
        align-items: center;
        gap: 10px;
    }

    .back-link {
        color: var(--accent-cyan);
        text-decoration: none;
        font-size: 13px;
        font-weight: 600;
    }

    .back-link:hover {
        text-decoration: underline;
    }

    h1 {
        font-size: 20px;
        font-weight: 700;
        color: var(--text-primary);
        margin: 0;
    }

    .event-count {
        font-size: 12px;
        color: var(--text-secondary);
        font-weight: 500;
    }

    .user-label {
        font-size: 12px;
        color: var(--text-secondary);
    }

    .btn-logout {
        padding: 5px 12px;
        border: 1px solid var(--separator);
        border-radius: 8px;
        background: var(--card-bg);
        color: var(--text-secondary);
        font-size: 12px;
        font-weight: 600;
        cursor: pointer;
    }

    .btn-logout:hover {
        color: var(--text-primary);
        border-color: var(--text-secondary);
    }

    .btn-theme {
        padding: 4px 10px;
        border: 1px solid var(--separator);
        border-radius: 8px;
        background: var(--card-bg);
        cursor: pointer;
        font-size: 16px;
        line-height: 1;
    }

    .btn-theme:hover {
        border-color: var(--accent-cyan);
    }

    .auto-toggle {
        display: flex;
        align-items: center;
        gap: 4px;
        font-size: 12px;
        color: var(--text-secondary);
        cursor: pointer;
    }

    .auto-toggle input {
        accent-color: var(--accent-cyan);
    }

    .empty {
        padding: 60px 20px;
        text-align: center;
        color: var(--text-secondary);
        font-size: 14px;
        background: var(--card-bg);
        border-radius: 14px;
        border: 1px solid var(--separator);
    }

    .empty-icon {
        font-size: 40px;
        margin-bottom: 12px;
    }

    .empty-sub {
        font-size: 12px;
        margin-top: 6px;
        opacity: 0.7;
    }

    .detector-status-bar {
        display: flex;
        gap: 10px;
        margin-bottom: 16px;
        flex-wrap: wrap;
    }

    .detector-chip {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 8px 14px;
        background: var(--card-bg);
        border: 1px solid var(--separator);
        border-radius: 10px;
        font-size: 12px;
    }

    .detector-chip.online {
        border-color: color-mix(
            in srgb,
            var(--accent-green, #22c55e) 40%,
            transparent
        );
    }

    .status-dot {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: #666;
        flex-shrink: 0;
    }

    .status-dot.online {
        background: #22c55e;
        box-shadow: 0 0 6px rgba(34, 197, 94, 0.5);
    }

    .det-name {
        font-weight: 700;
        color: var(--text-primary);
    }

    .det-addr {
        color: var(--text-secondary);
        font-family: monospace;
        font-size: 11px;
    }

    .det-seen {
        color: var(--text-secondary);
        font-size: 11px;
        font-style: italic;
    }

    .events-list {
        background: var(--card-bg);
        border: 1px solid var(--separator);
        border-radius: 14px;
        overflow-x: auto;
    }

    .filter-bar {
        display: flex;
        gap: 12px;
        align-items: center;
        margin-bottom: 14px;
    }

    .filter-label {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 12px;
        font-weight: 600;
        color: var(--text-secondary);
    }

    .filter-input {
        padding: 5px 10px;
        border: 1px solid var(--separator);
        border-radius: 8px;
        background: var(--card-bg);
        color: var(--text-primary);
        font-size: 12px;
        font-family: inherit;
    }

    .filter-input:focus {
        outline: none;
        border-color: var(--accent-cyan);
    }

    .events-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 12px;
    }

    .events-table thead {
        position: sticky;
        top: 0;
        z-index: 1;
    }

    .events-table th {
        padding: 10px 12px;
        text-align: left;
        font-size: 11px;
        font-weight: 700;
        color: var(--text-secondary);
        text-transform: uppercase;
        letter-spacing: 0.3px;
        border-bottom: 1px solid var(--separator);
        background: var(--card-bg);
        white-space: nowrap;
    }

    .events-table td {
        padding: 8px 12px;
        border-bottom: 1px solid
            color-mix(in srgb, var(--separator) 50%, transparent);
        color: var(--text-primary);
        white-space: nowrap;
    }

    .events-table tbody tr:hover {
        background: color-mix(in srgb, var(--accent-cyan) 5%, transparent);
    }

    .events-table tbody tr:last-child td {
        border-bottom: none;
    }

    .col-time {
        font-variant-numeric: tabular-nums;
        color: var(--text-secondary) !important;
        font-size: 11px;
    }

    .col-target {
        font-weight: 700;
    }

    .col-mono {
        font-family: monospace;
        font-variant-numeric: tabular-nums;
        font-size: 11px;
    }

    .confidence-badge {
        padding: 2px 8px;
        border-radius: 6px;
        font-size: 11px;
        font-weight: 700;
    }

    .confidence-badge.high {
        background: color-mix(in srgb, #ff4444 15%, var(--card-bg));
        color: #ff4444;
    }

    .confidence-badge.medium {
        background: color-mix(in srgb, #ffaa00 15%, var(--card-bg));
        color: #ffaa00;
    }

    .confidence-badge.low {
        background: color-mix(
            in srgb,
            var(--text-secondary) 12%,
            var(--card-bg)
        );
        color: var(--text-secondary);
    }

    .detector-label {
        font-size: 11px;
        font-weight: 600;
        color: var(--accent-cyan);
        background: color-mix(in srgb, var(--accent-cyan) 12%, var(--card-bg));
        padding: 1px 6px;
        border-radius: 4px;
    }
</style>
