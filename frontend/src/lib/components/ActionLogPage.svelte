<script lang="ts">
  import { onMount } from "svelte";
  import {
    authStore,
    authFetch,
    verifyToken,
    logout,
    isAdmin,
  } from "../store/authStore";
  import { API_BASE } from "../utils/api";
  import LoginPage from "./LoginPage.svelte";

  interface DBlockerConfig {
    signal_gps: boolean;
    signal_ctrl: boolean;
  }

  interface ActionLog {
    id: number;
    timestamp: string;
    username: string;
    action: string;
    dblocker_id: number;
    dblocker_name: string;
    config: DBlockerConfig[];
  }

  let logs = $state<ActionLog[]>([]);
  let total = $state(0);
  let loading = $state(false);
  let page = $state(0);
  const pageSize = 30;

  let authorized = $state(false);
  let darkMode = $state(true);

  // Default: today
  let fromDate = $state(todayStr());
  let toDate = $state(todayStr());

  function todayStr(): string {
    const d = new Date();
    return d.toISOString().slice(0, 10);
  }

  function loadTheme() {
    try {
      const stored = localStorage.getItem("app-settings");
      if (stored) {
        const parsed = JSON.parse(stored);
        darkMode = parsed.theme === "dark";
      } else {
        darkMode = true; // default dark
      }
    } catch {
      darkMode = true;
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
    // Persist to same key as dashboard
    try {
      const stored = localStorage.getItem("app-settings");
      const settings = stored ? JSON.parse(stored) : {};
      settings.theme = darkMode ? "dark" : "light";
      localStorage.setItem("app-settings", JSON.stringify(settings));
    } catch { /* ignore */ }
  }

  onMount(() => {
    loadTheme();
    if ($authStore.token) {
      verifyToken().then((valid) => {
        if (valid && isAdmin()) {
          authorized = true;
          fetchLogs();
        }
      });
    }
  });

  function dblockerLabel(log: ActionLog): string {
    return log.dblocker_name ? `${log.dblocker_name} (#${log.dblocker_id})` : `#${log.dblocker_id}`;
  }

  async function fetchLogs() {
    loading = true;
    try {
      const res = await authFetch(
        `${API_BASE}/api/logs?from=${fromDate}&to=${toDate}&limit=${pageSize}&offset=${page * pageSize}`
      );
      if (res.ok) {
        const json = await res.json();
        logs = json.data || [];
        total = json.total || 0;
      }
    } catch {
      console.error("Failed to fetch logs");
    } finally {
      loading = false;
    }
  }

  function applyFilter() {
    page = 0;
    fetchLogs();
  }

  function changePage(newPage: number) {
    page = newPage;
    fetchLogs();
  }

  let totalPages = $derived(Math.ceil(total / pageSize));

  function actionLabel(action: string): string {
    const labels: Record<string, string> = {
      config_update: "Config Update",
      turn_off_all: "Turn Off All",
      preset_on: "Preset ON",
      create_schedule: "Create Schedule",
      scheduled_config_update: "Scheduled Update",
    };
    return labels[action] || action;
  }

  function actionClass(action: string): string {
    if (action.startsWith("scheduled")) return "auto";
    if (action === "turn_off_all") return "off";
    if (action === "preset_on" || action === "config_update") return "on";
    return "neutral";
  }

  function localTzLabel(): string {
    const offset = -(new Date().getTimezoneOffset());
    const sign = offset >= 0 ? "+" : "-";
    const h = Math.floor(Math.abs(offset) / 60).toString().padStart(2, "0");
    const m = (Math.abs(offset) % 60).toString().padStart(2, "0");
    return `GMT${sign}${h}:${m}`;
  }

  function formatTimestamp(ts: string): string {
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

  function configSummary(config: DBlockerConfig[]): string {
    if (!config || config.length === 0) return "—";
    const parts: string[] = [];
    for (let i = 0; i < config.length; i++) {
      const c = config[i];
      if (c.signal_ctrl || c.signal_gps) {
        let s = `S${i + 1}:`;
        if (c.signal_ctrl) s += "RC";
        if (c.signal_ctrl && c.signal_gps) s += "+";
        if (c.signal_gps) s += "GPS";
        parts.push(s);
      }
    }
    return parts.length > 0 ? parts.join(" ") : "All OFF";
  }
</script>

{#if !$authStore.token}
  <LoginPage />
{:else if !authorized}
  <div class="log-page">
    <div class="access-denied">
      <h2>Access Denied</h2>
      <p>Only admin users can view action logs.</p>
      <a href="/dashboard" class="back-link">← Back to Dashboard</a>
    </div>
  </div>
{:else}
<div class="log-page">
  <header class="log-header">
    <div class="header-left">
      <a href="/dashboard" class="back-link">← Dashboard</a>
      <h1>Action Logs</h1>
    </div>
    <div class="header-right">
      <button class="btn-theme" onclick={toggleTheme} title="Toggle theme">
        {#if darkMode}☀{:else}🌙{/if}
      </button>
      <span class="user-label">{$authStore.user?.username}</span>
      <button class="btn-logout" onclick={logout}>Logout</button>
    </div>
  </header>

  <div class="controls">
    <div class="date-filter">
      <label>
        <span>From</span>
        <input type="date" bind:value={fromDate} />
      </label>
      <label>
        <span>To</span>
        <input type="date" bind:value={toDate} />
      </label>
      <button class="btn-filter" onclick={applyFilter}>Filter</button>
    </div>
    <div class="total-label">{total} entries</div>
  </div>

  {#if loading}
    <div class="loading">Loading...</div>
  {:else if logs.length === 0}
    <div class="empty">No action logs found</div>
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Time ({localTzLabel()})</th>
            <th>User</th>
            <th>Action</th>
            <th>DBlocker</th>
            <th>Config</th>
          </tr>
        </thead>
        <tbody>
          {#each logs as log (log.id)}
            <tr>
              <td class="cell-id">{log.id}</td>
              <td class="cell-time">{formatTimestamp(log.timestamp)}</td>
              <td class="cell-user">
                {#if log.username === "_service"}
                  <span class="badge service">assistant</span>
                {:else}
                  <span class="badge user">{log.username}</span>
                {/if}
              </td>
              <td>
                <span class="action-badge {actionClass(log.action)}">
                  {actionLabel(log.action)}
                </span>
              </td>
              <td class="cell-id">{dblockerLabel(log)}</td>
              <td class="cell-config">{configSummary(log.config)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    {#if totalPages > 1}
      <div class="pagination">
        <button
          class="page-btn"
          disabled={page === 0}
          onclick={() => changePage(page - 1)}
        >← Prev</button>
        <span class="page-info">Page {page + 1} of {totalPages}</span>
        <button
          class="page-btn"
          disabled={page >= totalPages - 1}
          onclick={() => changePage(page + 1)}
        >Next →</button>
      </div>
    {/if}
  {/if}
</div>
{/if}

<style>
  .log-page {
    max-width: 960px;
    margin: 0 auto;
    padding: 24px 16px;
    min-height: 100vh;
  }

  .access-denied {
    text-align: center;
    padding: 60px 20px;
    color: var(--text-secondary);
  }

  .access-denied h2 {
    font-size: 20px;
    color: var(--text-primary);
    margin-bottom: 8px;
  }

  .access-denied p {
    font-size: 14px;
    margin-bottom: 20px;
  }

  .log-header {
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
    transition: border-color 0.15s;
  }

  .btn-theme:hover {
    border-color: var(--accent-cyan);
  }

  .controls {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    flex-wrap: wrap;
    gap: 8px;
  }

  .date-filter {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .date-filter label {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .date-filter span {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .date-filter input[type="date"] {
    padding: 6px 10px;
    border: 1px solid var(--separator);
    border-radius: 8px;
    background: var(--card-bg);
    color: var(--text-primary);
    font-size: 13px;
    outline: none;
  }

  .date-filter input[type="date"]:focus {
    border-color: var(--accent-cyan);
  }

  .btn-filter {
    padding: 6px 16px;
    border: none;
    border-radius: 8px;
    background: var(--accent-cyan);
    color: #000;
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
    transition: opacity 0.15s;
  }

  .btn-filter:hover {
    opacity: 0.85;
  }

  .total-label {
    font-size: 12px;
    color: var(--text-secondary);
    font-weight: 500;
  }

  .loading,
  .empty {
    padding: 40px;
    text-align: center;
    color: var(--text-secondary);
    font-size: 14px;
    background: var(--card-bg);
    border-radius: 14px;
    border: 1px solid var(--separator);
  }

  .table-wrap {
    overflow-x: auto;
    border-radius: 14px;
    border: 1px solid var(--separator);
    background: var(--card-bg);
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }

  thead {
    background: color-mix(in srgb, var(--card-bg) 90%, var(--bg-elevated) 10%);
  }

  th {
    padding: 10px 12px;
    text-align: left;
    font-size: 11px;
    font-weight: 700;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    border-bottom: 1px solid var(--separator);
    white-space: nowrap;
  }

  td {
    padding: 10px 12px;
    border-bottom: 1px solid color-mix(in srgb, var(--separator) 50%, transparent);
    color: var(--text-primary);
    vertical-align: middle;
  }

  tr:last-child td {
    border-bottom: none;
  }

  tr:hover td {
    background: color-mix(in srgb, var(--accent-cyan) 4%, transparent);
  }

  .cell-time {
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    font-size: 12px;
    color: var(--text-secondary);
  }

  .cell-id {
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }

  .cell-config {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-secondary);
    font-family: monospace;
  }

  .badge {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 6px;
    font-size: 11px;
    font-weight: 600;
  }

  .badge.user {
    background: color-mix(in srgb, var(--accent-blue) 15%, var(--card-bg));
    color: var(--accent-blue);
  }

  .badge.service {
    background: color-mix(in srgb, var(--accent-green) 15%, var(--card-bg));
    color: var(--accent-green);
  }

  .action-badge {
    display: inline-block;
    padding: 3px 8px;
    border-radius: 6px;
    font-size: 11px;
    font-weight: 700;
    white-space: nowrap;
  }

  .action-badge.on {
    background: color-mix(in srgb, var(--accent-cyan) 15%, var(--card-bg));
    color: var(--accent-cyan);
  }

  .action-badge.off {
    background: color-mix(in srgb, #ff6b6b 12%, var(--card-bg));
    color: #ff6b6b;
  }

  .action-badge.auto {
    background: color-mix(in srgb, var(--accent-green) 15%, var(--card-bg));
    color: var(--accent-green);
  }

  .action-badge.neutral {
    background: color-mix(in srgb, var(--text-secondary) 10%, var(--card-bg));
    color: var(--text-secondary);
  }

  .pagination {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 12px;
    margin-top: 16px;
  }

  .page-btn {
    padding: 6px 14px;
    border: 1px solid var(--separator);
    border-radius: 8px;
    background: var(--card-bg);
    color: var(--text-primary);
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s;
  }

  .page-btn:hover:not(:disabled) {
    border-color: var(--accent-cyan);
    color: var(--accent-cyan);
  }

  .page-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .page-info {
    font-size: 12px;
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }
</style>
