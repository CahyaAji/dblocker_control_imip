<script lang="ts">
    import { onMount } from "svelte";
    import { authFetch } from "../store/authStore";
    import { API_BASE } from "../utils/api";

    let holdSeconds = $state(30);
    let autoBlocker = $state(true);
    let autoCamera = $state(true);
    let minConfidence = $state(0);
    let minSignalStrength = $state(0);
    let saving = $state(false);
    let error = $state("");
    let success = $state("");

    onMount(async () => {
        try {
            const res = await authFetch(`${API_BASE}/api/detectors/settings`);
            if (res.ok) {
                const json = await res.json();
                holdSeconds = json.data.hold_seconds;
                autoBlocker = json.data.auto_blocker ?? true;
                autoCamera = json.data.auto_camera ?? true;
                minConfidence = json.data.min_confidence ?? 0;
                minSignalStrength = json.data.min_signal_strength ?? 0;
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
                body: JSON.stringify({
                    hold_seconds: holdSeconds,
                    auto_blocker: autoBlocker,
                    auto_camera: autoCamera,
                    min_confidence: minConfidence,
                    min_signal_strength: minSignalStrength,
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

    function adjust(delta: number) {
        const next = holdSeconds + delta;
        if (next >= 5 && next <= 3600) holdSeconds = next;
    }

    function adjustConf(delta: number) {
        const next = minConfidence + delta;
        if (next >= 0 && next <= 100) minConfidence = next;
    }

    function adjustSig(delta: number) {
        const next = minSignalStrength + delta;
        // range: -120 (very weak) to 0 (disabled)
        if (next >= -120 && next <= 0) minSignalStrength = next;
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

    <div class="threshold-row">
        <div class="label">
            <span class="dot dot-conf"></span>
            Min confidence
        </div>
        <div class="stepper">
            <button class="step-btn" onclick={() => adjustConf(-5)}>−</button>
            <span class="value">{minConfidence === 0 ? "Off" : `${minConfidence}%`}</span>
            <button class="step-btn" onclick={() => adjustConf(5)}>+</button>
        </div>
    </div>

    <div class="threshold-row">
        <div class="label">
            <span class="dot dot-sig"></span>
            Min signal strength
        </div>
        <div class="stepper">
            <button class="step-btn" onclick={() => adjustSig(-5)}>−</button>
            <span class="value">{minSignalStrength === 0 ? "Off" : `${minSignalStrength} dB`}</span>
            <button class="step-btn" onclick={() => adjustSig(5)}>+</button>
        </div>
    </div>

    <div class="toggle-row">
        <div class="label">
            <span class="dot dot-blocker"></span>
            Auto-activate blocker
        </div>
        <button
            class="toggle-btn {autoBlocker ? 'on' : 'off'}"
            onclick={() => (autoBlocker = !autoBlocker)}
            aria-label="Toggle auto blocker"
        >
            <span class="toggle-thumb"></span>
        </button>
    </div>

    <div class="toggle-row">
        <div class="label">
            <span class="dot dot-camera"></span>
            Auto-move camera
        </div>
        <button
            class="toggle-btn {autoCamera ? 'on' : 'off'}"
            onclick={() => (autoCamera = !autoCamera)}
            aria-label="Toggle auto camera"
        >
            <span class="toggle-thumb"></span>
        </button>
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

    .toggle-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 8px 0;
        border-top: 1px solid color-mix(in srgb, var(--separator) 40%, transparent);
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

    .dot-blocker { background: #e74c3c; }
    .dot-camera  { background: #f39c12; }
    .dot-conf    { background: #9b59b6; }
    .dot-sig     { background: #1abc9c; }

    .toggle-btn {
        position: relative;
        width: 40px;
        height: 22px;
        border-radius: 999px;
        border: none;
        cursor: pointer;
        transition: background 0.2s;
        flex-shrink: 0;
        padding: 0;
    }

    .toggle-btn.on  { background: var(--accent-green, #27ae60); }
    .toggle-btn.off { background: color-mix(in srgb, var(--separator) 80%, transparent); }

    .toggle-thumb {
        position: absolute;
        top: 3px;
        left: 3px;
        width: 16px;
        height: 16px;
        border-radius: 50%;
        background: #fff;
        transition: transform 0.2s;
    }

    .toggle-btn.on .toggle-thumb  { transform: translateX(18px); }
    .toggle-btn.off .toggle-thumb { transform: translateX(0); }

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
