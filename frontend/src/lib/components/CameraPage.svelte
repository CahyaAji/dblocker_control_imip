<script lang="ts">
  import { onMount } from "svelte";
  import { settings } from "../store/configStore";

  // ── Types ─────────────────────────────────────────────────────────────────
  interface DeviceInfo {
    id: number;
    name: string;
    lat: number;
    lng: number;
    normal_ip: string;
    thermal_ip: string;
    pantilt_ip: string;
    zoom_ip: string;
  }

  type ViewType = "normal" | "thermal";

  // ── State ─────────────────────────────────────────────────────────────────
  let devices = $state<DeviceInfo[]>([]);
  let focusedSlot = $state<number | null>(null);
  let focusedView = $state<ViewType>("normal");
  let streamLoading = $state(false);
  let loading = $state(false);
  let error = $state<string | null>(null);

  // ── Helpers ───────────────────────────────────────────────────────────────
  const focusedDevice = $derived(
    focusedSlot !== null ? (devices[focusedSlot] ?? null) : null,
  );

  const streamUrl = (id: number, view: ViewType) =>
    `/cam/devices/${id}/stream/${view}`;

  // ── Lifecycle ─────────────────────────────────────────────────────────────
  onMount(async () => {
    if (typeof document !== "undefined") {
      document.documentElement.classList.toggle(
        "dark",
        $settings.theme === "dark",
      );
    }
    loading = true;
    try {
      const res = await fetch("/cam/devices");
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const json = await res.json();
      devices = json.data ?? [];
    } catch (e: unknown) {
      error = e instanceof Error ? e.message : "Failed to load devices";
    } finally {
      loading = false;
    }
  });

  // ── Reactive theme ────────────────────────────────────────────────────────
  $effect(() => {
    if (typeof document !== "undefined") {
      document.documentElement.classList.toggle(
        "dark",
        $settings.theme === "dark",
      );
    }
  });

  // ── Actions ───────────────────────────────────────────────────────────────
  const toggleTheme = () => {
    $settings.theme = $settings.theme === "dark" ? "light" : "dark";
  };

  const openFocus = (slot: number) => {
    focusedSlot = slot;
    focusedView = "normal";
    streamLoading = true;
  };

  const exitFocus = () => {
    focusedSlot = null;
  };

  const PTZ_SPEED = 20;  // -100..100
  const ZOOM_SPEED = 20; // -100..100

  // Send one continuous pan/tilt command (fire-and-forget).
  const sendPTZ = (pan: number, tilt: number) => {
    if (!focusedDevice) return;
    fetch(`/cam/devices/${focusedDevice.id}/pantilt/continuous`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ pan, tilt }),
    }).catch(() => {});
  };

  // Send one continuous zoom command (fire-and-forget).
  const sendZoom = (zoom: number) => {
    if (!focusedDevice || focusedView === "thermal") return;
    fetch(`/cam/devices/${focusedDevice.id}/zoom/continuous`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ zoom }),
    }).catch(() => {});
  };

  const directionMap: Record<"up" | "down" | "left" | "right", [number, number]> = {
    up:    [0,           PTZ_SPEED],
    down:  [0,          -PTZ_SPEED],
    left:  [-PTZ_SPEED,  0],
    right: [PTZ_SPEED,   0],
  };

  const startPTZ = (direction: "up" | "down" | "left" | "right", e: PointerEvent) => {
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    const [pan, tilt] = directionMap[direction];
    sendPTZ(pan, tilt);
  };

  const stopPTZ = () => sendPTZ(0, 0);

  const startZoom = (direction: "in" | "out", e: PointerEvent) => {
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    sendZoom(direction === "in" ? ZOOM_SPEED : -ZOOM_SPEED);
  };

  const stopZoom = () => sendZoom(0);

  // ── Recording ─────────────────────────────────────────────────────────────
  interface RecordStatusData {
    recording: boolean;
    cam?: string;
    filename?: string;
    started_at?: string;
    elapsed_seconds?: number;
    remaining_seconds?: number;
  }

  let recStatus = $state<RecordStatusData>({ recording: false });
  let recPollTimer: ReturnType<typeof setInterval> | null = null;

  // Poll record status every second while in focused view.
  $effect(() => {
    if (focusedSlot !== null && focusedDevice) {
      pollRecStatus();
      recPollTimer = setInterval(pollRecStatus, 1000);
      return () => {
        if (recPollTimer !== null) {
          clearInterval(recPollTimer);
          recPollTimer = null;
        }
        recStatus = { recording: false };
      };
    }
  });

  const pollRecStatus = async () => {
    if (!focusedDevice) return;
    try {
      const res = await fetch(`/cam/devices/${focusedDevice.id}/record/status`);
      if (res.ok) recStatus = await res.json();
    } catch {}
  };

  const startRecording = async () => {
    if (!focusedDevice) return;
    try {
      await fetch(`/cam/devices/${focusedDevice.id}/record/start`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ cam: focusedView }),
      });
      await pollRecStatus();
    } catch {}
  };

  const stopRecording = async () => {
    if (!focusedDevice) return;
    try {
      await fetch(`/cam/devices/${focusedDevice.id}/record/stop`, { method: "POST" });
      await pollRecStatus();
    } catch {}
  };

  const formatSecs = (s: number) => {
    const m = Math.floor(s / 60);
    const sec = s % 60;
    return `${String(m).padStart(2, "0")}:${String(sec).padStart(2, "0")}`;
  };
</script>

<div class="cam-page">
  <!-- ── Top Bar ─────────────────────────────────────────────────────────── -->
  <header class="top-bar">
    <div class="top-bar-left">
      <a href="/dashboard" class="back-btn" title="Back to dashboard">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          ><polyline points="15 18 9 12 15 6" /></svg
        >
        Back
      </a>
      <span class="page-title">Camera View</span>
    </div>

    <div class="top-bar-center">
      {#if loading}
        <span class="status-text">Loading devices…</span>
      {:else if error}
        <span class="error-text">{error}</span>
      {/if}
    </div>

    <div class="top-bar-right">
      <button class="icon-btn" onclick={toggleTheme} aria-label="Toggle theme" title="Toggle theme">
        {#if $settings.theme === "dark"}
          <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>
        {:else}
          <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
        {/if}
      </button>
    </div>
  </header>

  <!-- ── Main Content ───────────────────────────────────────────────────── -->
  <main class="cam-main">
    {#if !loading && !error && devices.length === 0}
      <!-- ── Empty state ────────────────────────────────────────────────── -->
      <div class="empty-state">
        <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="empty-icon"><path d="M23 7l-7 5 7 5V7z"/><rect x="1" y="5" width="15" height="14" rx="2" ry="2"/></svg>
        <p>No devices available</p>
        <p class="empty-hint">Configure devices using environment variables on the vision server.</p>
      </div>

    {:else if focusedSlot === null}
      <!-- ── Dual-Window View: device 1 left, device 2 right ──────────────── -->
      <div class="dual-grid">
        {#each [0, 1] as slot}
          {@const dev = devices[slot] ?? null}
          <button
            type="button"
            class="stream-window"
            onclick={() => { if (dev) openFocus(slot); }}
            aria-label={dev ? `Focus ${dev.name}` : `No device ${slot + 1}`}
            disabled={!dev}
          >
            {#if dev}
              <img
                src={streamUrl(dev.id, "normal")}
                alt="{dev.name} normal stream"
                class="stream-img"
              />
              <div class="stream-label">
                <span class="label-badge normal-badge">{dev.name}</span>
                <span class="click-hint">Click to focus</span>
              </div>
            {:else}
              <div class="stream-empty-inner"><span>No device {slot + 1}</span></div>
            {/if}
          </button>
        {/each}
      </div>

    {:else if focusedDevice}
      <!-- ── Focused View: stream left, controls right ──────────────────── -->
      <div class="focused-layout">

        <!-- Stream -->
        <div class="focused-stream-wrap">
          <img
            src={streamUrl(focusedDevice.id, focusedView)}
            alt="{focusedDevice.name} {focusedView} stream"
            class="stream-img"
            onload={() => (streamLoading = false)}
            onerror={() => (streamLoading = false)}
          />
          {#if streamLoading}
            <div class="stream-loading-overlay">
              <div class="stream-spinner"></div>
            </div>
          {/if}
          <div class="focused-badge">
            {#if focusedView === "normal"}
              <span class="label-badge normal-badge">Normal</span>
            {:else}
              <span class="label-badge thermal-badge">Thermal</span>
            {/if}
          </div>
          {#if recStatus.recording}
            <div class="rec-overlay">
              <span class="rec-dot"></span>
              <span class="rec-time">{formatSecs(recStatus.elapsed_seconds ?? 0)}</span>
            </div>
          {/if}
        </div>

        <!-- Control panel -->
        <aside class="control-panel">

          <!-- Back -->
          <button type="button" class="exit-btn" onclick={exitFocus}
            disabled={recStatus.recording}
            title={recStatus.recording ? "Stop recording first" : ""}>
            <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>
            Dual View
          </button>

          <!-- View toggle -->
          <div class="ctrl-section">
            <p class="ctrl-label">View</p>
            <div class="view-toggle">
              <button type="button" class="view-btn" class:active={focusedView === "normal"}
                disabled={recStatus.recording}
                onclick={() => { if (focusedView !== "normal") { streamLoading = true; focusedView = "normal"; } }}>Normal</button>
              <button type="button" class="view-btn" class:active={focusedView === "thermal"}
                disabled={recStatus.recording}
                onclick={() => { if (focusedView !== "thermal") { streamLoading = true; focusedView = "thermal"; } }}>Thermal</button>
            </div>
          </div>

          <!-- PTZ D-pad -->
          <div class="ctrl-section">
            <p class="ctrl-label">Move</p>
            <div class="dpad">
              <div class="dpad-row">
                <button type="button" class="dpad-btn"
                  onpointerdown={(e) => startPTZ("up", e)}
                  onpointerup={stopPTZ}
                  onpointercancel={stopPTZ}
                  aria-label="Tilt up" title="Up">
                  <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="18 15 12 9 6 15"/></svg>
                </button>
              </div>
              <div class="dpad-row">
                <button type="button" class="dpad-btn"
                  onpointerdown={(e) => startPTZ("left", e)}
                  onpointerup={stopPTZ}
                  onpointercancel={stopPTZ}
                  aria-label="Pan left" title="Left">
                  <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>
                </button>
                <div class="dpad-center" aria-hidden="true"></div>
                <button type="button" class="dpad-btn"
                  onpointerdown={(e) => startPTZ("right", e)}
                  onpointerup={stopPTZ}
                  onpointercancel={stopPTZ}
                  aria-label="Pan right" title="Right">
                  <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
                </button>
              </div>
              <div class="dpad-row">
                <button type="button" class="dpad-btn"
                  onpointerdown={(e) => startPTZ("down", e)}
                  onpointerup={stopPTZ}
                  onpointercancel={stopPTZ}
                  aria-label="Tilt down" title="Down">
                  <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>
                </button>
              </div>
            </div>
          </div>

          <!-- Zoom -->
          <div class="ctrl-section">
            <p class="ctrl-label">Zoom</p>
            <div class="zoom-row">
              <button type="button" class="zoom-btn"
                onpointerdown={(e) => startZoom("in", e)}
                onpointerup={stopZoom}
                onpointercancel={stopZoom}
                disabled={focusedView === "thermal"}
                aria-label="Zoom in">
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/><line x1="11" y1="8" x2="11" y2="14"/><line x1="8" y1="11" x2="14" y2="11"/></svg>
                Zoom In
              </button>
              <button type="button" class="zoom-btn"
                onpointerdown={(e) => startZoom("out", e)}
                onpointerup={stopZoom}
                onpointercancel={stopZoom}
                disabled={focusedView === "thermal"}
                aria-label="Zoom out">
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/><line x1="8" y1="11" x2="14" y2="11"/></svg>
                Zoom Out
              </button>
            </div>
          </div>

          <!-- Record -->
          <div class="ctrl-section">
            <p class="ctrl-label">Record</p>
            {#if recStatus.recording}
              <div class="rec-info">
                <span class="rec-elapsed">{formatSecs(recStatus.elapsed_seconds ?? 0)}</span>
                <span class="rec-remain">−{formatSecs(recStatus.remaining_seconds ?? 0)}</span>
              </div>
              <button type="button" class="rec-stop-btn" onclick={stopRecording}>
                ■ Stop
              </button>
            {:else}
              <button type="button" class="rec-start-btn" onclick={startRecording}>
                ● Record
              </button>
            {/if}
          </div>

        </aside>
      </div>
    {/if}
  </main>
</div>

<style>
  /* ── Page layout ──────────────────────────────────────────────────────── */
  .cam-page {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
    background: transparent;
  }

  /* ── Top bar ─────────────────────────────────────────────────────────── */
  .top-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 16px;
    background: linear-gradient(
      180deg,
      color-mix(in srgb, var(--card-bg) 92%, var(--accent-cyan) 8%) 0%,
      var(--card-bg) 100%
    );
    border-bottom: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
    backdrop-filter: blur(8px);
    flex-shrink: 0;
    gap: 12px;
    z-index: 10;
  }

  .top-bar-left {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }

  .top-bar-center {
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 1;
  }

  .top-bar-right {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .back-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 10px;
    border-radius: 10px;
    background: color-mix(in srgb, var(--card-bg) 86%, var(--bg-elevated) 14%);
    border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    color: var(--text-primary);
    font-size: 13px;
    font-weight: 500;
    text-decoration: none;
    transition: all 0.2s ease;
    white-space: nowrap;
  }

  .back-btn:hover {
    transform: translateY(-1px);
    border-color: color-mix(in srgb, var(--accent-blue) 40%, transparent);
    box-shadow: 0 4px 12px rgba(19, 134, 217, 0.15);
  }

  .page-title {
    font-size: 16px;
    font-weight: 700;
    color: var(--text-primary);
    white-space: nowrap;
  }

  .device-selector-wrap {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .device-label {
    font-size: 13px;
    color: var(--text-secondary);
    font-weight: 500;
    white-space: nowrap;
  }

  .device-select {
    padding: 6px 10px;
    border-radius: 10px;
    border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    background: color-mix(in srgb, var(--card-bg) 90%, var(--bg-elevated) 10%);
    color: var(--text-primary);
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    outline: none;
    transition: border-color 0.2s;
    min-width: 160px;
  }

  .device-select:focus {
    border-color: color-mix(in srgb, var(--accent-cyan) 60%, transparent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent-cyan) 14%, transparent);
  }

  .status-text {
    font-size: 13px;
    color: var(--text-secondary);
  }

  .error-text {
    font-size: 13px;
    color: #f87171;
  }

  .icon-btn {
    background: color-mix(in srgb, var(--card-bg) 86%, var(--bg-elevated) 14%);
    border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    border-radius: 10px;
    cursor: pointer;
    color: var(--text-primary);
    padding: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s ease;
  }

  .icon-btn:hover {
    transform: translateY(-1px);
    border-color: color-mix(in srgb, var(--accent-cyan) 45%, transparent);
    box-shadow: 0 4px 12px rgba(19, 182, 217, 0.18);
  }

  /* ── Main area ───────────────────────────────────────────────────────── */
  .cam-main {
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  /* ── Dual grid ───────────────────────────────────────────────────────── */
  .dual-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    grid-template-rows: 1fr;
    gap: 12px;
    padding: 12px;
    flex: 1;
    min-height: 0;
  }

  .stream-window {
    position: relative;
    overflow: hidden;
    border-radius: var(--radius-md);
    border: 2px solid color-mix(in srgb, var(--separator) 60%, transparent);
    background: #000;
    cursor: pointer;
    padding: 0;
    transition: border-color 0.2s, box-shadow 0.2s, transform 0.15s;
    display: flex;
    align-items: stretch;
    min-height: 0;
    width: 100%;
  }

  .stream-window:hover {
    border-color: color-mix(in srgb, var(--accent-cyan) 70%, transparent);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent-cyan) 18%, transparent), var(--shadow-md);
    transform: scale(1.006);
  }

  .stream-window:focus-visible {
    outline: 2px solid var(--accent-cyan);
    outline-offset: 2px;
  }

  .stream-img {
    width: 100%;
    height: 100%;
    object-fit: contain;
    display: block;
    background: #000;
  }

  .stream-label {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 10px 12px;
    background: linear-gradient(to top, rgba(0, 0, 0, 0.7) 0%, transparent 100%);
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    pointer-events: none;
  }

  .label-badge {
    font-size: 12px;
    font-weight: 700;
    color: #fff;
    padding: 3px 10px;
    border-radius: 6px;
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .normal-badge  { background: color-mix(in srgb, var(--accent-blue) 82%, #000 18%); }
  .thermal-badge { background: color-mix(in srgb, #f97316 82%, #000 18%); }

  .click-hint {
    font-size: 11px;
    color: rgba(255, 255, 255, 0.65);
  }

  .stream-empty-inner {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #0a0a0a;
    color: rgba(255, 255, 255, 0.3);
    font-size: 13px;
  }

  /* ── Focused layout ──────────────────────────────────────────────────── */
  .focused-layout {
    display: flex;
    flex-direction: row;
    gap: 12px;
    padding: 12px;
    flex: 1;
    min-height: 0;
  }

  .focused-stream-wrap {
    position: relative;
    flex: 1;
    border-radius: var(--radius-md);
    overflow: hidden;
    background: #000;
    border: 2px solid color-mix(in srgb, var(--separator) 60%, transparent);
    box-shadow: var(--shadow-md);
    min-width: 0;
    min-height: 0;
  }

  .focused-badge {
    position: absolute;
    top: 12px;
    left: 12px;
    pointer-events: none;
  }

  .stream-loading-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(0, 0, 0, 0.45);
    z-index: 2;
  }

  .stream-spinner {
    width: 38px;
    height: 38px;
    border: 3px solid rgba(255, 255, 255, 0.18);
    border-top-color: rgba(255, 255, 255, 0.85);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* ── Control panel ───────────────────────────────────────────────────── */
  .control-panel {
    display: flex;
    flex-direction: column;
    gap: 10px;
    width: 176px;
    flex-shrink: 0;
    overflow-y: auto;
  }

  .exit-btn {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 7px 12px;
    border-radius: 10px;
    border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    background: color-mix(in srgb, var(--card-bg) 86%, var(--bg-elevated) 14%);
    color: var(--text-secondary);
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;
    width: 100%;
  }

  .exit-btn:hover {
    color: var(--text-primary);
    border-color: color-mix(in srgb, var(--accent-blue) 40%, transparent);
    box-shadow: 0 4px 10px rgba(19, 134, 217, 0.14);
    transform: translateY(-1px);
  }

  /* Section card */
  .ctrl-section {
    background: var(--card-bg);
    border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    border-radius: var(--radius-md);
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    box-shadow: var(--shadow-sm);
  }

  .ctrl-label {
    margin: 0;
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-secondary);
  }

  /* View toggle */
  .view-toggle {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .view-btn {
    width: 100%;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    background: color-mix(in srgb, var(--bg-elevated) 90%, transparent);
    color: var(--text-secondary);
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    text-align: center;
  }

  .view-btn:hover {
    color: var(--text-primary);
    background: color-mix(in srgb, var(--accent-blue) 10%, var(--bg-elevated) 90%);
    border-color: color-mix(in srgb, var(--accent-blue) 40%, transparent);
  }

  .view-btn.active {
    background: color-mix(in srgb, var(--accent-cyan) 18%, var(--card-bg) 82%);
    border-color: color-mix(in srgb, var(--accent-cyan) 60%, transparent);
    color: var(--text-primary);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent-cyan) 22%, transparent);
  }

  /* D-pad */
  .dpad {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .dpad-row {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .dpad-center {
    width: 40px;
    height: 40px;
    border-radius: 8px;
    background: color-mix(in srgb, var(--separator) 40%, transparent);
    flex-shrink: 0;
  }

  .dpad-btn {
    width: 40px;
    height: 40px;
    border-radius: 8px;
    border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    background: color-mix(in srgb, var(--bg-elevated) 90%, transparent);
    color: var(--text-primary);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.15s ease;
    flex-shrink: 0;
  }

  .dpad-btn:hover {
    background: color-mix(in srgb, var(--accent-blue) 18%, var(--bg-elevated) 82%);
    border-color: color-mix(in srgb, var(--accent-blue) 50%, transparent);
    transform: scale(1.1);
    box-shadow: 0 4px 10px rgba(19, 134, 217, 0.2);
  }

  .dpad-btn:active {
    transform: scale(0.92);
    background: color-mix(in srgb, var(--accent-blue) 28%, var(--bg-elevated) 72%);
  }

  /* Zoom */
  .zoom-row {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .zoom-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
    background: color-mix(in srgb, var(--bg-elevated) 90%, transparent);
    color: var(--text-secondary);
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s ease;
    width: 100%;
  }

  .zoom-btn:hover {
    color: var(--text-primary);
    background: color-mix(in srgb, var(--accent-cyan) 14%, var(--bg-elevated) 86%);
    border-color: color-mix(in srgb, var(--accent-cyan) 50%, transparent);
    transform: scale(1.03);
  }

  .zoom-btn:active {
    transform: scale(0.96);
  }

  .zoom-btn:disabled {
    opacity: 0.38;
    cursor: not-allowed;
    pointer-events: none;
  }

  /* ── Record UI ───────────────────────────────────────────────────────── */
  .rec-overlay {
    position: absolute;
    top: 12px;
    right: 12px;
    display: flex;
    align-items: center;
    gap: 5px;
    background: rgba(0, 0, 0, 0.55);
    border-radius: 8px;
    padding: 4px 10px;
    pointer-events: none;
    z-index: 3;
  }

  .rec-dot {
    width: 8px;
    height: 8px;
    background: #ef4444;
    border-radius: 50%;
    flex-shrink: 0;
    animation: rec-blink 1s ease-in-out infinite;
  }

  @keyframes rec-blink {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.15; }
  }

  .rec-time {
    font-size: 13px;
    font-weight: 700;
    color: #fff;
    font-variant-numeric: tabular-nums;
  }

  .rec-info {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-variant-numeric: tabular-nums;
    margin-bottom: 2px;
  }

  .rec-elapsed {
    font-size: 13px;
    font-weight: 700;
    color: #ef4444;
  }

  .rec-remain {
    font-size: 11px;
    color: var(--text-secondary);
  }

  .rec-start-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid color-mix(in srgb, #ef4444 45%, transparent);
    background: color-mix(in srgb, #ef4444 10%, var(--bg-elevated) 90%);
    color: #ef4444;
    font-size: 13px;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.15s ease;
    letter-spacing: 0.02em;
  }

  .rec-start-btn:hover {
    background: color-mix(in srgb, #ef4444 20%, var(--bg-elevated) 80%);
    border-color: color-mix(in srgb, #ef4444 70%, transparent);
    transform: scale(1.03);
  }

  .rec-stop-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid color-mix(in srgb, #ef4444 65%, transparent);
    background: color-mix(in srgb, #ef4444 24%, var(--bg-elevated) 76%);
    color: #fff;
    font-size: 13px;
    font-weight: 700;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .rec-stop-btn:hover {
    background: color-mix(in srgb, #ef4444 36%, var(--bg-elevated) 64%);
    transform: scale(1.03);
  }

  /* Disabled states for exit and view buttons */
  .exit-btn:disabled,
  .view-btn:disabled {
    opacity: 0.38;
    cursor: not-allowed;
    pointer-events: none;
  }

  /* ── Empty state ─────────────────────────────────────────────────────── */
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 12px;
    color: var(--text-secondary);
    text-align: center;
    padding: 24px;
  }

  .empty-icon {
    opacity: 0.4;
  }

  .empty-state p {
    margin: 0;
    font-size: 15px;
    font-weight: 500;
  }

  .empty-hint {
    font-size: 13px !important;
    font-weight: 400 !important;
    opacity: 0.75;
    max-width: 360px;
  }

  /* ── Responsive ──────────────────────────────────────────────────────── */
  @media (max-width: 640px) {
    .dual-grid {
      grid-template-columns: 1fr;
    }

    .page-title {
      display: none;
    }

    .focused-layout {
      flex-direction: column;
    }

    .control-panel {
      width: 100%;
      flex-direction: row;
      flex-wrap: wrap;
      overflow-y: visible;
    }

    .ctrl-section {
      flex: 1;
      min-width: 130px;
    }
  }
</style>
