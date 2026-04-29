<script lang="ts">
    import { onMount } from "svelte";
    import { authFetch } from "../store/authStore";
    import { API_BASE } from "../utils/api";

    let holdSeconds = $state(30);
    let saving = $state(false);
    let error = $state("");
    let success = $state("");

    onMount(async () => {
        try {
            const res = await authFetch(`${API_BASE}/api/detectors/settings`);
            if (res.ok) {
                const json = await res.json();
                holdSeconds = json.data.hold_seconds;
            }
        } catch {
            error = "Failed to load detection settings";
        }
    });

    async function save() {
        error = "";
        success = "";
        saving = true;
        try {
            const res = await authFetch(`${API_BASE}/api/detectors/settings`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ hold_seconds: holdSeconds }),
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

    function adjust(delta: number) {
        const next = holdSeconds + delta;
        if (next >= 5 && next <= 3600) holdSeconds = next;
    }
</script>

<div class="card">
    <div class="card-header">Detection Auto-Off</div>

    <div class="threshold-row">
        <div class="label">
            <span class="dot"></span>
            Blocker hold time
        </div>
        <div class="stepper">
            <button class="step-btn" onclick={() => adjust(-5)}>−</button>
            <span class="value">{holdSeconds} s</span>
            <button class="step-btn" onclick={() => adjust(5)}>+</button>
        </div>
    </div>

    <div class="footer">
        {#if error}
            <span class="msg error">{error}</span>
        {:else if success}
            <span class="msg success">{success}</span>
        {:else}
            <span class="msg hint">Stays ON after last detection</span>
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
        background: var(--accent-blue);
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
        min-width: 56px;
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

    .msg.hint { color: var(--text-secondary); }
    .msg.error { color: #e74c3c; }
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

    .btn-save:hover {
        background: color-mix(in srgb, var(--card-bg) 78%, var(--accent-blue) 22%);
    }

    .btn-save:disabled {
        opacity: 0.5;
        cursor: default;
    }
</style>
