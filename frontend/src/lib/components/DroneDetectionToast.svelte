<script lang="ts">
    import { onMount } from "svelte";
    import {
        subscribeBridge,
        unsubscribeBridge,
        bridgeStore,
    } from "../store/bridgeStore";

    interface Toast {
        id: number;
        targetName: string;
        detector: string;
        heading: number;
        distance: number;
        confidence: number;
        timerId: ReturnType<typeof setTimeout>;
    }

    let toasts = $state<Toast[]>([]);
    let nextId = 0;
    let lastPayload = "";

    const MAX_TOASTS = 5;
    const AUTO_DISMISS_MS = 8000;

    function headingLabel(deg: number): string {
        const dirs = ["N", "NE", "E", "SE", "S", "SW", "W", "NW"];
        return dirs[Math.round(((deg % 360) + 360) % 360 / 45) % 8];
    }

    function playAlertSound() {
        try {
            const ctx = new AudioContext();
            const tones = [880, 660];
            let time = ctx.currentTime;
            let lastOsc: OscillatorNode | null = null;
            for (const freq of tones) {
                const osc = ctx.createOscillator();
                const gain = ctx.createGain();
                osc.connect(gain);
                gain.connect(ctx.destination);
                osc.type = "sine";
                osc.frequency.value = freq;
                gain.gain.setValueAtTime(0, time);
                gain.gain.linearRampToValueAtTime(0.25, time + 0.01);
                gain.gain.exponentialRampToValueAtTime(0.001, time + 0.18);
                osc.start(time);
                osc.stop(time + 0.18);
                time += 0.2;
                lastOsc = osc;
            }
            if (lastOsc) lastOsc.onended = () => ctx.close();
        } catch { /* AudioContext not available */ }
    }

    function dismiss(id: number) {
        const t = toasts.find((x) => x.id === id);
        if (t) clearTimeout(t.timerId);
        toasts = toasts.filter((x) => x.id !== id);
    }

    $effect(() => {
        const payload = $bridgeStore["detections/live"];
        if (!payload || payload === lastPayload) return;
        lastPayload = payload;

        let d: Record<string, unknown>;
        try {
            d = JSON.parse(payload);
        } catch {
            return;
        }

        const targetName = String(d.target_name || d.unique_id || "Unknown");
        const detector = String(d.detector ?? "Unknown");
        const heading = ((Number(d.heading ?? 0) % 360) + 360) % 360;
        const distance = Number(d.distance ?? 0);
        const confidence = Number(d.confidence ?? 0);

        const id = nextId++;
        const timerId = setTimeout(() => dismiss(id), AUTO_DISMISS_MS);

        playAlertSound();
        toasts = [{ id, targetName, detector, heading, distance, confidence, timerId }, ...toasts].slice(0, MAX_TOASTS);
    });

    onMount(() => {
        subscribeBridge();
        return () => {
            toasts.forEach((t) => clearTimeout(t.timerId));
            unsubscribeBridge();
        };
    });
</script>

{#if toasts.length > 0}
    <div class="toast-stack" role="alert" aria-live="assertive">
        {#each toasts as toast (toast.id)}
            <div class="toast">
                <div class="toast-icon">
                    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <circle cx="12" cy="12" r="10"/>
                        <circle cx="12" cy="12" r="6"/>
                        <circle cx="12" cy="12" r="2"/>
                        <line x1="12" y1="2" x2="12" y2="4"/>
                        <line x1="12" y1="20" x2="12" y2="22"/>
                        <line x1="2" y1="12" x2="4" y2="12"/>
                        <line x1="20" y1="12" x2="22" y2="12"/>
                    </svg>
                </div>
                <div class="toast-body">
                    <div class="toast-title">Drone Detected</div>
                    <div class="toast-target">{toast.targetName}</div>
                    <div class="toast-meta">
                        <span>{toast.detector}</span>
                        <span class="dot">·</span>
                        <span>{headingLabel(toast.heading)} {Math.round(toast.heading)}°</span>
                        {#if toast.distance > 0}
                            <span class="dot">·</span>
                            <span>{toast.distance.toFixed(0)} m</span>
                        {/if}
                        {#if toast.confidence > 0}
                            <span class="dot">·</span>
                            <span>{Math.round(toast.confidence * 100)}%</span>
                        {/if}
                    </div>
                </div>
                <button
                    class="toast-close"
                    type="button"
                    aria-label="Dismiss"
                    onclick={() => dismiss(toast.id)}
                >✕</button>
            </div>
        {/each}
    </div>
{/if}

<style>
    .toast-stack {
        position: fixed;
        top: 16px;
        left: 50%;
        transform: translateX(-50%);
        z-index: 9999;
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 8px;
        pointer-events: none;
        width: min(380px, calc(100vw - 32px));
    }

    .toast {
        pointer-events: auto;
        display: flex;
        align-items: flex-start;
        gap: 10px;
        padding: 12px 12px 12px 14px;
        border-radius: 12px;
        background: color-mix(in srgb, #1a0a0a 92%, transparent);
        border: 1.5px solid rgba(239, 68, 68, 0.65);
        box-shadow:
            0 0 0 1px rgba(239, 68, 68, 0.15),
            0 8px 24px rgba(0, 0, 0, 0.45);
        backdrop-filter: blur(10px);
        animation: toast-in 0.25s cubic-bezier(0.34, 1.56, 0.64, 1) both,
                   toast-glow 1.6s ease-in-out 3;
        color: #fef2f2;
    }

    .toast-icon {
        flex-shrink: 0;
        color: #f87171;
        margin-top: 1px;
        animation: icon-pulse 1s ease-in-out infinite alternate;
    }

    .toast-body {
        flex: 1;
        min-width: 0;
        display: flex;
        flex-direction: column;
        gap: 3px;
    }

    .toast-title {
        font-size: 11px;
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: 0.08em;
        color: #f87171;
    }

    .toast-target {
        font-size: 14px;
        font-weight: 600;
        color: #fef2f2;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .toast-meta {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: 4px;
        font-size: 11px;
        color: #fca5a5;
        opacity: 0.88;
    }

    .dot {
        opacity: 0.5;
    }

    .toast-close {
        flex-shrink: 0;
        background: none;
        border: none;
        cursor: pointer;
        color: #fca5a5;
        font-size: 12px;
        padding: 2px 4px;
        line-height: 1;
        border-radius: 4px;
        opacity: 0.6;
        transition: opacity 0.15s;
        margin-top: -2px;
    }

    .toast-close:hover {
        opacity: 1;
    }

    @keyframes toast-in {
        from {
            opacity: 0;
            transform: translateY(-12px) scale(0.96);
        }
        to {
            opacity: 1;
            transform: translateY(0) scale(1);
        }
    }

    @keyframes toast-glow {
        0%, 100% { box-shadow: 0 0 0 1px rgba(239, 68, 68, 0.15), 0 8px 24px rgba(0, 0, 0, 0.45); }
        50%       { box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.35), 0 8px 24px rgba(239, 68, 68, 0.25); }
    }

    @keyframes icon-pulse {
        from { opacity: 0.7; }
        to   { opacity: 1; }
    }

    @media (max-width: 768px) {
        .toast-stack {
            top: auto;
            bottom: 16px;
            left: 16px;
            right: 16px;
            transform: none;
            width: auto;
        }
    }
</style>
