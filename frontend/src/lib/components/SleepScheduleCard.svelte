<script lang="ts">
    import { onMount } from "svelte";
    import { authFetch } from "../store/authStore";
    import { API_BASE } from "../utils/api";

    const timezones = [
        "-12:00", "-11:00", "-10:00", "-09:00", "-08:00", "-07:00", "-06:00",
        "-05:00", "-04:00", "-03:00", "-02:00", "-01:00", "+00:00",
        "+01:00", "+02:00", "+03:00", "+04:00", "+05:00", "+05:30", "+05:45",
        "+06:00", "+06:30", "+07:00", "+08:00", "+09:00", "+09:30",
        "+10:00", "+11:00", "+12:00", "+13:00", "+14:00",
    ];

    const hours = Array.from({ length: 24 }, (_, i) => i.toString().padStart(2, "0"));
    const minutes = Array.from({ length: 60 }, (_, i) => i.toString().padStart(2, "0"));

    let enabled = $state(false);
    let sleepHour = $state("22");
    let sleepMinute = $state("00");
    let wakeHour = $state("06");
    let wakeMinute = $state("00");
    let timezone = $state("+07:00");
    let saving = $state(false);
    let error = $state("");
    let success = $state("");

    onMount(async () => {
        try {
            const res = await authFetch(`${API_BASE}/api/dblockers/sleep-schedule`);
            if (res.ok) {
                const json = await res.json();
                const d = json.data;
                enabled = d.enabled;
                [sleepHour, sleepMinute] = d.sleep_time.split(":");
                [wakeHour, wakeMinute] = d.wake_time.split(":");
                timezone = d.timezone;
            }
        } catch {
            error = "Failed to load sleep schedule";
        }
    });

    async function save() {
        error = "";
        success = "";
        saving = true;
        try {
            const res = await authFetch(`${API_BASE}/api/dblockers/sleep-schedule`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    enabled,
                    sleep_time: `${sleepHour}:${sleepMinute}`,
                    wake_time: `${wakeHour}:${wakeMinute}`,
                    timezone,
                }),
            });
            if (res.ok) {
                success = "Saved";
                setTimeout(() => (success = ""), 2000);
            } else {
                const json = await res.json().catch(() => ({}));
                error = json.error || "Failed to save";
            }
        } catch {
            error = "Network error";
        } finally {
            saving = false;
        }
    }
</script>

<div class="card">
    <div class="card-header">
        <span>Sleep / Wake Schedule</span>
        <label class="toggle-switch">
            <input type="checkbox" bind:checked={enabled} />
            <span class="slider"></span>
        </label>
    </div>

    <div class="time-row">
        <div class="row-label">
            <span class="dot sleep"></span>
            Sleep at
        </div>
        <div class="time-picker">
            <select bind:value={sleepHour} disabled={!enabled}>
                {#each hours as h}
                    <option value={h}>{h}</option>
                {/each}
            </select>
            <span class="sep">:</span>
            <select bind:value={sleepMinute} disabled={!enabled}>
                {#each minutes as m}
                    <option value={m}>{m}</option>
                {/each}
            </select>
        </div>
    </div>

    <div class="time-row">
        <div class="row-label">
            <span class="dot wake"></span>
            Wake at
        </div>
        <div class="time-picker">
            <select bind:value={wakeHour} disabled={!enabled}>
                {#each hours as h}
                    <option value={h}>{h}</option>
                {/each}
            </select>
            <span class="sep">:</span>
            <select bind:value={wakeMinute} disabled={!enabled}>
                {#each minutes as m}
                    <option value={m}>{m}</option>
                {/each}
            </select>
        </div>
    </div>

    <div class="tz-row">
        <span class="tz-label">Timezone</span>
        <select class="tz-select" bind:value={timezone} disabled={!enabled}>
            {#each timezones as tz}
                <option value={tz}>UTC{tz}</option>
            {/each}
        </select>
    </div>

    <div class="footer">
        {#if error}
            <span class="msg error">{error}</span>
        {:else if success}
            <span class="msg success">{success}</span>
        {:else}
            <span class="msg hint">Applies to all DBlockers</span>
        {/if}
        <button class="btn-save" onclick={save} disabled={saving}>
            {saving ? "Saving…" : "Save"}
        </button>
    </div>
</div>

<style>
    .card {
        background-color: var(--card-bg);
        border-radius: var(--radius-md);
        padding: 14px 12px;
        border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
        box-shadow: var(--shadow-sm);
    }

    .card-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        font-size: 14px;
        font-weight: 700;
        margin-bottom: 12px;
    }

    .time-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 8px 0;
        border-top: 1px solid color-mix(in srgb, var(--separator) 50%, transparent);
    }

    .row-label {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 13px;
        font-weight: 600;
        color: var(--text-secondary);
    }

    .dot {
        width: 8px;
        height: 8px;
        border-radius: 50%;
    }
    .dot.sleep { background: #7c6af7; }
    .dot.wake  { background: #f7c948; }

    .time-picker {
        display: flex;
        align-items: center;
        gap: 3px;
    }

    .sep {
        font-size: 15px;
        font-weight: 700;
        color: var(--text-primary);
    }

    select {
        padding: 5px 8px;
        border-radius: 8px;
        border: 1px solid var(--separator);
        background: var(--card-bg);
        color: var(--text-primary);
        font-size: 13px;
        font-weight: 600;
        outline: none;
        cursor: pointer;
    }

    select:focus {
        border-color: var(--accent-blue);
    }

    select:disabled {
        opacity: 0.4;
        cursor: default;
    }

    .tz-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 8px 0;
        border-top: 1px solid color-mix(in srgb, var(--separator) 50%, transparent);
    }

    .tz-label {
        font-size: 13px;
        font-weight: 600;
        color: var(--text-secondary);
    }

    .tz-select {
        font-size: 12px;
    }

    .footer {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-top: 10px;
        gap: 8px;
    }

    .msg {
        font-size: 11px;
        font-weight: 600;
    }
    .msg.hint    { color: var(--text-secondary); }
    .msg.error   { color: #e74c3c; }
    .msg.success { color: var(--accent-green); }

    .btn-save {
        font-size: 12px;
        padding: 6px 16px;
        border-radius: 999px;
        border: 1px solid color-mix(in srgb, var(--accent-blue) 38%, transparent);
        color: var(--accent-blue);
        background: color-mix(in srgb, var(--card-bg) 90%, var(--accent-blue) 10%);
        font-weight: 700;
        cursor: pointer;
        transition: all 0.2s ease;
        white-space: nowrap;
    }

    /* Toggle switch */
    .toggle-switch {
        position: relative;
        display: inline-block;
        width: 38px;
        height: 20px;
        cursor: pointer;
    }

    .toggle-switch input {
        opacity: 0;
        width: 0;
        height: 0;
    }

    .toggle-switch .slider {
        position: absolute;
        top: 0; left: 0; right: 0; bottom: 0;
        background: var(--separator);
        border-radius: 20px;
        transition: 0.3s;
    }

    .toggle-switch .slider:before {
        position: absolute;
        content: "";
        height: 16px;
        width: 16px;
        left: 2px;
        bottom: 2px;
        background: var(--text-primary);
        border-radius: 50%;
        transition: 0.3s;
    }

    .toggle-switch input:checked + .slider {
        background: var(--accent-blue);
    }

    .toggle-switch input:checked + .slider:before {
        transform: translateX(18px);
    }
</style>
