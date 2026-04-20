<script lang="ts">
    import { onMount } from "svelte";
    import { authFetch } from "../store/authStore";
    import { API_BASE } from "../utils/api";

    let fanOnTemp = $state(45);
    let fanOffTemp = $state(35);
    let saving = $state(false);
    let error = $state("");
    let success = $state("");

    onMount(async () => {
        try {
            const res = await authFetch(`${API_BASE}/api/dblockers/fan-thresholds`);
            if (res.ok) {
                const json = await res.json();
                fanOnTemp = json.data.fan_on_temp;
                fanOffTemp = json.data.fan_off_temp;
            }
        } catch {
            error = "Failed to load fan settings";
        }
    });

    async function save() {
        error = "";
        success = "";

        if (fanOnTemp - fanOffTemp < 5) {
            error = "ON temperature must be at least 5°C higher than OFF temperature";
            return;
        }

        saving = true;
        try {
            const res = await authFetch(`${API_BASE}/api/dblockers/fan-thresholds`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    fan_on_temp: fanOnTemp,
                    fan_off_temp: fanOffTemp,
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

    function adjustOn(delta: number) {
        const next = fanOnTemp + delta;
        if (next - fanOffTemp >= 5) fanOnTemp = next;
    }

    function adjustOff(delta: number) {
        const next = fanOffTemp + delta;
        if (fanOnTemp - next >= 5) fanOffTemp = next;
    }
</script>

<div class="card">
    <div class="card-header">Fan Temperature</div>

    <div class="threshold-row">
        <div class="label">
            <span class="dot on"></span>
            Turn ON above
        </div>
        <div class="stepper">
            <button class="step-btn" onclick={() => adjustOn(-1)}>−</button>
            <span class="value">{fanOnTemp}°C</span>
            <button class="step-btn" onclick={() => adjustOn(1)}>+</button>
        </div>
    </div>

    <div class="threshold-row">
        <div class="label">
            <span class="dot off"></span>
            Turn OFF below
        </div>
        <div class="stepper">
            <button class="step-btn" onclick={() => adjustOff(-1)}>−</button>
            <span class="value">{fanOffTemp}°C</span>
            <button class="step-btn" onclick={() => adjustOff(1)}>+</button>
        </div>
    </div>

    <div class="footer">
        {#if error}
            <span class="msg error">{error}</span>
        {:else if success}
            <span class="msg success">{success}</span>
        {:else}
            <span class="msg hint">Gap: {fanOnTemp - fanOffTemp}°C (min 5°C)</span>
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
        font-size: 14px;
        font-weight: 700;
        margin-bottom: 12px;
    }

    .threshold-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 8px 0;
    }

    .threshold-row + .threshold-row {
        border-top: 1px solid color-mix(in srgb, var(--separator) 50%, transparent);
    }

    .label {
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

    .dot.on {
        background: var(--accent-green);
    }

    .dot.off {
        background: #e74c3c;
    }

    .stepper {
        display: flex;
        align-items: center;
        gap: 4px;
        background: color-mix(in srgb, var(--card-bg) 85%, var(--bg-elevated) 15%);
        border: 1px solid color-mix(in srgb, var(--separator) 60%, transparent);
        border-radius: 999px;
        padding: 2px;
    }

    .step-btn {
        width: 28px;
        height: 28px;
        border-radius: 50%;
        border: none;
        background: transparent;
        color: var(--text-primary);
        font-size: 16px;
        font-weight: 700;
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: background 0.15s;
    }

    .step-btn:hover {
        background: color-mix(in srgb, var(--separator) 60%, transparent);
    }

    .value {
        min-width: 48px;
        text-align: center;
        font-size: 14px;
        font-weight: 700;
        color: var(--text-primary);
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

    .msg.hint {
        color: var(--text-secondary);
    }

    .msg.error {
        color: #e74c3c;
    }

    .msg.success {
        color: var(--accent-green);
    }

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

    .btn-save:hover:not(:disabled) {
        transform: translateY(-1px);
        box-shadow: 0 6px 12px rgba(19, 134, 217, 0.18);
    }

    .btn-save:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
</style>
