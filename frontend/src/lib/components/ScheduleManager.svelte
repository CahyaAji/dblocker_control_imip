<script lang="ts">
  import { onMount } from "svelte";
  import { authFetch } from "../store/authStore";
  import {
    dblockerStore,
    fetchDBlockers,
    type DBlocker,
    type DBlockerConfig,
  } from "../store/dblockerStore";
  import { API_BASE } from "../utils/api";

  interface Schedule {
    id: number;
    dblocker_id: number;
    dblocker?: DBlocker;
    config: DBlockerConfig[];
    time: string;
    timezone: string;
    enabled: boolean;
    created_at: string;
  }

  let schedules = $state<Schedule[]>([]);
  let loading = $state(false);
  let error = $state("");

  // Form state
  let selectedDBlockerId = $state(0);
  let scheduleHour = $state("08");
  let scheduleMinute = $state("00");
  let selectedTimezone = $state("+07:00");
  let sectorConfig = $state<DBlockerConfig[]>(
    Array.from({ length: 6 }, () => ({ signal_gps: false, signal_ctrl: false }))
  );

  const hours = Array.from({ length: 24 }, (_, i) => i.toString().padStart(2, "0"));
  const minutes = Array.from({ length: 60 }, (_, i) => i.toString().padStart(2, "0"));
  const timezones = [
    "-12:00", "-11:00", "-10:00", "-09:00", "-08:00", "-07:00", "-06:00",
    "-05:00", "-04:00", "-03:00", "-02:00", "-01:00", "+00:00",
    "+01:00", "+02:00", "+03:00", "+04:00", "+05:00", "+05:30", "+05:45",
    "+06:00", "+06:30", "+07:00", "+08:00", "+09:00", "+09:30",
    "+10:00", "+11:00", "+12:00", "+13:00", "+14:00",
  ];

  let dblockers = $derived([...$dblockerStore].sort((a, b) => a.id - b.id));

  onMount(async () => {
    await fetchDBlockers();
    await loadSchedules();
  });

  async function loadSchedules() {
    try {
      const res = await authFetch(`${API_BASE}/api/schedules`);
      if (res.ok) {
        const json = await res.json();
        schedules = json.data || [];
      }
    } catch {
      console.error("Failed to load schedules");
    }
  }

  async function createSchedule(e: Event) {
    e.preventDefault();
    if (!selectedDBlockerId) {
      error = "Please select a DBlocker";
      return;
    }
    error = "";
    loading = true;

    try {
      const res = await authFetch(`${API_BASE}/api/schedules`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          dblocker_id: selectedDBlockerId,
          config: sectorConfig,
          time: `${scheduleHour}:${scheduleMinute}`,
          timezone: selectedTimezone,
        }),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        error = data.error || "Failed to create schedule";
      } else {
        // Reset form
        sectorConfig = Array.from({ length: 6 }, () => ({
          signal_gps: false,
          signal_ctrl: false,
        }));
        await loadSchedules();
      }
    } catch {
      error = "Failed to create schedule";
    } finally {
      loading = false;
    }
  }

  async function toggleSchedule(id: number) {
    try {
      await authFetch(`${API_BASE}/api/schedules/${id}/toggle`, {
        method: "PUT",
      });
      await loadSchedules();
    } catch {
      console.error("Failed to toggle schedule");
    }
  }

  async function deleteSchedule(id: number) {
    if (!confirm("Delete this schedule?")) return;
    try {
      await authFetch(`${API_BASE}/api/schedules/${id}`, {
        method: "DELETE",
      });
      await loadSchedules();
    } catch {
      console.error("Failed to delete schedule");
    }
  }

  function toggleSector(index: number, key: keyof DBlockerConfig) {
    sectorConfig = sectorConfig.map((cfg, i) =>
      i === index ? { ...cfg, [key]: !cfg[key] } : cfg
    );
  }

  function getDblockerName(schedule: Schedule): string {
    return schedule.dblocker?.name || `DBlocker #${schedule.dblocker_id}`;
  }

  function utcToLocal(utcTime: string, tz: string): string {
    const [h, m] = utcTime.split(":").map(Number);
    const sign = tz[0] === "-" ? -1 : 1;
    const [tzH, tzM] = tz.slice(1).split(":").map(Number);
    const offset = sign * (tzH * 60 + tzM);
    let total = h * 60 + m + offset;
    total = ((total % 1440) + 1440) % 1440;
    return `${String(Math.floor(total / 60)).padStart(2, "0")}:${String(total % 60).padStart(2, "0")}`;
  }

  function countActive(config: DBlockerConfig[]): number {
    return config.filter((c) => c.signal_ctrl || c.signal_gps).length;
  }
</script>

<div class="scheduler">
  <!-- Create Form -->
  <form class="form-card" onsubmit={createSchedule}>
    <div class="form-title">New Schedule</div>

    {#if error}
      <div class="error-msg">{error}</div>
    {/if}

    <div class="form-group">
      <label for="dblocker-select">DBlocker</label>
      <select id="dblocker-select" bind:value={selectedDBlockerId}>
        <option value={0} disabled>Select DBlocker...</option>
        {#each dblockers as db (db.id)}
          <option value={db.id}>{db.name}</option>
        {/each}
      </select>
    </div>

    <div class="form-group">
      <label>Time (local 24h)</label>
      <div class="time-picker">
        <select bind:value={scheduleHour}>
          {#each hours as h}
            <option value={h}>{h}</option>
          {/each}
        </select>
        <span class="time-sep">:</span>
        <select bind:value={scheduleMinute}>
          {#each minutes as m}
            <option value={m}>{m}</option>
          {/each}
        </select>
        <select class="tz-select" bind:value={selectedTimezone}>
          {#each timezones as tz}
            <option value={tz}>UTC{tz}</option>
          {/each}
        </select>
      </div>
    </div>

    <div class="form-group">
      <label>Sector Config</label>
      <div class="sector-grid">
        {#each sectorConfig as cfg, i}
          <div class="sector-item">
            <div class="sector-label">S{i + 1}</div>
            <div class="sector-toggles">
              <label class="toggle-label">
                <input
                  type="checkbox"
                  checked={cfg.signal_ctrl}
                  onchange={() => toggleSector(i, "signal_ctrl")}
                />
                <span class="toggle-text">RC</span>
              </label>
              <label class="toggle-label">
                <input
                  type="checkbox"
                  checked={cfg.signal_gps}
                  onchange={() => toggleSector(i, "signal_gps")}
                />
                <span class="toggle-text">GPS</span>
              </label>
            </div>
          </div>
        {/each}
      </div>
    </div>

    <button type="submit" class="btn-create" disabled={loading}>
      {loading ? "Creating..." : "Add Schedule"}
    </button>
  </form>

  <!-- Schedule List -->
  <div class="schedule-list">
    {#each schedules as schedule (schedule.id)}
      <div class="schedule-item" class:disabled={!schedule.enabled}>
        <div class="schedule-header">
          <div class="schedule-info">
            <span class="schedule-time">{utcToLocal(schedule.time, schedule.timezone)}</span>
            <span class="schedule-tz">UTC{schedule.timezone}</span>
            <span class="schedule-name">{getDblockerName(schedule)}</span>
          </div>
          <div class="schedule-actions">
            <label class="switch small">
              <input
                type="checkbox"
                checked={schedule.enabled}
                onchange={() => toggleSchedule(schedule.id)}
              />
              <span class="slider"></span>
            </label>
            <button
              class="btn-delete"
              onclick={() => deleteSchedule(schedule.id)}
              title="Delete"
            >✕</button>
          </div>
        </div>
        <div class="schedule-sectors">
          {#each schedule.config as cfg, i}
            {#if cfg.signal_ctrl || cfg.signal_gps}
              <span class="sector-badge">
                S{i + 1}:
                {#if cfg.signal_ctrl}RC{/if}
                {#if cfg.signal_ctrl && cfg.signal_gps}+{/if}
                {#if cfg.signal_gps}GPS{/if}
              </span>
            {/if}
          {/each}
          {#if countActive(schedule.config) === 0}
            <span class="sector-badge off">All OFF</span>
          {/if}
        </div>
      </div>
    {:else}
      <div class="empty">No schedules configured</div>
    {/each}
  </div>
</div>

<style>
  .scheduler {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 10px 6px;
    flex: 1;
    overflow-y: auto;
    scrollbar-color: var(--separator) var(--bg-color);
    min-height: 0;
  }

  .form-card {
    background: linear-gradient(
      160deg,
      color-mix(in srgb, var(--card-bg) 90%, var(--accent-cyan) 10%) 0%,
      color-mix(in srgb, var(--card-bg) 92%, var(--accent-blue) 8%) 100%
    );
    border: 1px solid color-mix(in srgb, var(--separator) 72%, transparent);
    border-radius: 14px;
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .form-title {
    font-size: 13px;
    font-weight: 700;
    color: var(--text-primary);
  }

  .error-msg {
    background: color-mix(in srgb, #ff4444 15%, var(--card-bg));
    color: #ff6b6b;
    padding: 6px 10px;
    border-radius: 8px;
    font-size: 12px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .form-group label {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  select {
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid var(--separator);
    background: var(--card-bg);
    color: var(--text-primary);
    font-size: 13px;
    outline: none;
  }

  select:focus {
    border-color: var(--accent-cyan);
  }

  .time-picker {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .time-picker select {
    flex: 1;
    text-align: center;
  }

  .tz-select {
    margin-left: 6px;
    flex: 1.2 !important;
    font-size: 12px !important;
  }

  .time-sep {
    font-size: 16px;
    font-weight: 700;
    color: var(--text-primary);
  }

  .sector-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 6px;
  }

  .sector-item {
    background: var(--card-bg);
    border: 1px solid var(--separator);
    border-radius: 8px;
    padding: 6px 8px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .sector-label {
    font-size: 11px;
    font-weight: 700;
    color: var(--text-secondary);
  }

  .sector-toggles {
    display: flex;
    gap: 6px;
  }

  .toggle-label {
    display: flex;
    align-items: center;
    gap: 3px;
    cursor: pointer;
    font-size: 10px;
  }

  .toggle-label input[type="checkbox"] {
    width: 14px;
    height: 14px;
    accent-color: var(--accent-cyan);
    cursor: pointer;
  }

  .toggle-text {
    color: var(--text-secondary);
    font-weight: 600;
    font-size: 10px;
  }

  .btn-create {
    padding: 8px 14px;
    border: none;
    border-radius: 8px;
    background: var(--accent-cyan);
    color: #000;
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
    transition: opacity 0.2s;
  }

  .btn-create:hover {
    opacity: 0.85;
  }

  .btn-create:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .schedule-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .schedule-item {
    background: linear-gradient(
      160deg,
      color-mix(in srgb, var(--card-bg) 90%, var(--accent-cyan) 10%) 0%,
      color-mix(in srgb, var(--card-bg) 92%, var(--accent-blue) 8%) 100%
    );
    border: 1px solid color-mix(in srgb, var(--separator) 72%, transparent);
    border-radius: 12px;
    padding: 10px 12px;
    transition: opacity 0.2s;
  }

  .schedule-item.disabled {
    opacity: 0.45;
  }

  .schedule-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .schedule-info {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .schedule-time {
    font-size: 16px;
    font-weight: 700;
    color: var(--accent-cyan);
    font-variant-numeric: tabular-nums;
  }

  .schedule-tz {
    font-size: 10px;
    color: var(--text-secondary);
    opacity: 0.7;
    font-weight: 500;
  }

  .schedule-name {
    font-size: 12px;
    color: var(--text-secondary);
    font-weight: 500;
  }

  .schedule-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .btn-delete {
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 14px;
    padding: 2px 4px;
    border-radius: 4px;
    transition: all 0.15s;
  }

  .btn-delete:hover {
    color: #ff6b6b;
    background: rgba(255, 107, 107, 0.1);
  }

  .schedule-sectors {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    margin-top: 6px;
  }

  .sector-badge {
    font-size: 10px;
    font-weight: 600;
    padding: 2px 6px;
    border-radius: 4px;
    background: color-mix(in srgb, var(--accent-cyan) 15%, var(--card-bg));
    color: var(--accent-cyan);
  }

  .sector-badge.off {
    background: color-mix(in srgb, var(--text-secondary) 10%, var(--card-bg));
    color: var(--text-secondary);
  }

  .empty {
    padding: 18px;
    border-radius: 14px;
    border: 1px dashed var(--separator);
    color: var(--text-secondary);
    text-align: center;
    background: color-mix(
      in srgb,
      var(--card-bg) 88%,
      var(--bg-elevated) 12%
    );
  }

  /* Toggle switch (small) */
  .switch {
    position: relative;
    display: inline-block;
    width: 34px;
    height: 18px;
  }

  .switch input {
    opacity: 0;
    width: 0;
    height: 0;
  }

  .slider {
    position: absolute;
    cursor: pointer;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: var(--separator);
    transition: 0.3s;
    border-radius: 18px;
  }

  .slider:before {
    position: absolute;
    content: "";
    height: 14px;
    width: 14px;
    left: 2px;
    bottom: 2px;
    background-color: var(--text-primary);
    transition: 0.3s;
    border-radius: 50%;
  }

  input:checked + .slider {
    background-color: var(--accent-cyan);
  }

  input:checked + .slider:before {
    transform: translateX(16px);
  }
</style>
