<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import DblockerCardActions from "./DblockerCardActions.svelte";
    import DblockerSectorGrid from "./DblockerSectorGrid.svelte";
    import type { DBlocker, DBlockerConfig, SectorCurrents } from "../store/dblockerStore";
    import { updateDBlockerConfig, turnOffAll, presetOn, sleepDBlocker, rebootDBlocker, wakeDBlocker, expandedDblockerId } from "../store/dblockerStore";
    import { bridgeStore, subscribeBridge, unsubscribeBridge } from "../store/bridgeStore";
    import { API_BASE } from "../utils/api";

    const calculateCurrentA = (rawADC: number): number => {
        const VCC = 3.3;
        const voltage = rawADC * (VCC / 1023.0);
        const vZero = VCC / 2.0;
        const sensitivity = 0.0396;
        return (voltage - vZero) / sensitivity;
    };

    const calculateTemperatureC = (rawADC: number): number | null => {
        if (rawADC <= 0 || rawADC >= 1023) return null;
        const R_FIXED = 51000.0;
        const R0 = 100000.0;
        const B_COEFFICIENT = 3950.0;
        const T0 = 298.15;
        const resistance = R_FIXED * (1023.0 / rawADC - 1.0);
        let steinhart = resistance / R0;
        steinhart = Math.log(steinhart);
        steinhart /= B_COEFFICIENT;
        steinhart += 1.0 / T0;
        steinhart = 1.0 / steinhart;
        return steinhart - 273.15;
    };

    type ParsedRpt = {
        sectors: SectorCurrents[];
        temperatureC: number | null;
    };

    const parseRpt = (payload: string): ParsedRpt | null => {
        const [numericPart] = payload.split("|");
        if (!numericPart) return null;

        const values = numericPart
            .split(",")
            .map((v) => Number(v.trim()))
            .filter((v) => !Number.isNaN(v));

        if (values.length < 19) return null;

        const sectors = Array.from({ length: 6 }, (_, s) => ({
            gps: calculateCurrentA(values[s * 3]),
            ctrl1: calculateCurrentA(values[s * 3 + 1]),
            ctrl2: calculateCurrentA(values[s * 3 + 2]),
        }));

        return { sectors, temperatureC: calculateTemperatureC(values[18]) };
    };

    export let dblocker: DBlocker;

    $: staTopic = `dbl/${dblocker.serial_numb}/sta`;
    $: staPayload = $bridgeStore[staTopic] ?? null;
    $: staLabel =
        staPayload === null ? '—' :
        staPayload === 'OFF' ? 'OFFLINE' :
        staPayload === 'SLEEP' ? 'SLEEP' :
        staPayload.startsWith('ON') ? 'ONLINE' : staPayload;

    // --- /rpt topic: live current sensor data ---
    $: rptTopic = `dbl/${dblocker.serial_numb}/rpt`;
    $: rptPayload = $bridgeStore[rptTopic] ?? null;
    $: parsedRpt = rptPayload ? parseRpt(rptPayload) : null;
    $: liveTemperatureC = parsedRpt?.temperatureC ?? null;

    // --- Monitor status from backend ---
    let monitorErrors: string[] = [];
    let warningState: "normal" | "error" = "normal";
    let monitorPollTimer: ReturnType<typeof setInterval> | undefined;

    async function pollMonitorStatus() {
        try {
            const res = await fetch(`${API_BASE}/api/dblockers/monitor`);
            if (!res.ok) return;
            const json = await res.json();
            const allStatus: Record<string, { errors: string[] }> = json.data ?? {};
            const status = allStatus[dblocker.serial_numb];
            monitorErrors = status?.errors ?? [];
            warningState = monitorErrors.length > 0 ? "error" : "normal";
        } catch {
            // Ignore fetch errors
        }
    }

    $: warningTitle = warningState === "error"
        ? `Error: ${monitorErrors.join(', ')} current lower than expected`
        : "Normal";

    
    // Expansion is now controlled by the store
    $: isExpanded = $expandedDblockerId === dblocker.id;
    let showAdvancedActions = false;
    let hasEdited = false;

    // A local copy of the editable config
    let editableConfig: DBlockerConfig[] = [];

    // The store's configuration
    $: liveConfig = dblocker.config ?? [];

    // Sync editableConfig with liveConfig ONLY if not waiting for backend confirmation
    let waitingForBackend = false;
    $: {
        if (isExpanded && !hasEdited && !waitingForBackend) {
            editableConfig = liveConfig.map((c) => ({ ...c }));
        }
    }

    // A flag indicating whether the editable config differs from the live config
    $: canReadLastState =
        JSON.stringify(editableConfig) !== JSON.stringify(liveConfig);

    function toggleExpanded() {
        if ($expandedDblockerId === dblocker.id) {
            showAdvancedActions = false;
            expandedDblockerId.set(null);
        } else {
            editableConfig = liveConfig.map((c) => ({ ...c }));
            hasEdited = false;
            expandedDblockerId.set(dblocker.id);
        }
    }

    function toggleAdvanced() {
        showAdvancedActions = !showAdvancedActions;
    }

    function toggleEditableSignal(
        sectorIndex: number,
        signalKey: keyof DBlockerConfig,
    ) {
        editableConfig[sectorIndex][signalKey] =
            !editableConfig[sectorIndex][signalKey];
        editableConfig = editableConfig;
        hasEdited = true;
    }

    function reloadFromLatest() {
        editableConfig = liveConfig.map((c) => ({ ...c }));
        hasEdited = false;
    }

    async function applyConfig() {
        try {
            waitingForBackend = true;
            await updateDBlockerConfig(dblocker.id, editableConfig);
            // Backend snapshots currents at config-apply; start polling for monitor errors
            pollMonitorStatus();
            // Wait for backend/store to match our local config (max 5 seconds)
            let checkAttempts = 0;
            const checkMatch = () => {
                const isMatch =
                    JSON.stringify(dblocker.config) ===
                    JSON.stringify(editableConfig);
                if (isMatch || checkAttempts++ >= 50) {
                    hasEdited = false;
                    waitingForBackend = false;
                } else {
                    setTimeout(checkMatch, 100);
                }
            };
            checkMatch();
        } catch (err) {
            waitingForBackend = false;
            // Error already handled in updateDBlockerConfig
        }
    }

    async function handleAdvancedAction(action: "sleep" | "reboot" | "wake") {
        if (action === "sleep") {
            await sleepDBlocker(dblocker.id);
        } else if (action === "reboot") {
            await rebootDBlocker(dblocker.id);
        } else if (action === "wake") {
            await wakeDBlocker(dblocker.id);
        }
    }

    async function handleOffAll() {
        await turnOffAll(dblocker.id);
    }

    async function handlePresetOn() {
        await presetOn(dblocker.id);
    }

    onMount(() => {
        subscribeBridge();
        pollMonitorStatus();
        monitorPollTimer = setInterval(pollMonitorStatus, 3000);
    });
    onDestroy(() => {
        unsubscribeBridge();
        if (monitorPollTimer) clearInterval(monitorPollTimer);
    });
</script>

<div class="card" class:expanded={isExpanded} class:has-error={warningState === "error"}>
    <div class="card-header" class:has-error={warningState === "error"}>
        <div class="title-wrap">
            <div class="title-meta-wrap">
                <div class="card-title">{dblocker.name}</div>
                <span class="title-separator" aria-hidden="true">|</span>
                <div class="card-meta" class:status-on={staPayload?.startsWith('ON')} class:status-off={staPayload === 'OFF'} class:status-sleep={staPayload === 'SLEEP'}>
                    {staLabel}{#if liveTemperatureC !== null} | {liveTemperatureC.toFixed(1)}°C{/if}
                </div>
            </div>
            <div
                class="warning-indicator"
                class:is-error={warningState === "error"}
                title={warningTitle}
                aria-label={warningTitle}
            >
                {#if warningState === "error"}
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                        <path d="M12 3L2.6 19.2A1.2 1.2 0 003.64 21h16.72a1.2 1.2 0 001.04-1.8L12 3zm1 13h-2v-2h2v2zm0-4h-2V8h2v4z" />
                    </svg>
                {:else}
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                        <path d="M12 2a10 10 0 100 20 10 10 0 000-20zm4.2 7.3l-5.03 6.1a1 1 0 01-1.48.07l-2.2-2.2a1 1 0 011.41-1.42l1.42 1.43 4.25-5.15a1 1 0 011.54 1.27z" />
                    </svg>
                {/if}
            </div>
        </div>
    </div>
    <DblockerSectorGrid
        {isExpanded}
        {showAdvancedActions}
        {liveConfig}
        {editableConfig}
        onToggleSignal={toggleEditableSignal}
        onAdvancedAction={handleAdvancedAction}
    />

    <DblockerCardActions
        {isExpanded}
        {canReadLastState}
        {showAdvancedActions}
        hasPreset={Array.isArray(dblocker.preset_config) && dblocker.preset_config.length === 6}
        onReadLastState={reloadFromLatest}
        onApply={applyConfig}
        onPresetOn={handlePresetOn}
        onOffAll={handleOffAll}
        onToggleAdvanced={toggleAdvanced}
        onToggleExpanded={toggleExpanded}
    />
</div>

<style>
    .card {
        margin: 0;
        border-radius: var(--radius-lg);
        padding: 14px;
        overflow: visible;
        background: linear-gradient(
            155deg,
            color-mix(in srgb, var(--card-bg) 74%, var(--bg-elevated) 26%) 0%,
            color-mix(in srgb, var(--card-bg) 90%, var(--accent-cyan) 10%) 50%,
            color-mix(in srgb, var(--card-bg) 82%, var(--accent-green) 18%) 100%
        );
        border: 1px solid
            color-mix(in srgb, var(--separator) 74%, var(--accent-blue) 26%);
        box-shadow:
            0 10px 24px rgba(18, 35, 48, 0.1),
            inset 0 1px 0 rgba(255, 255, 255, 0.22);
        transition:
            transform 0.2s ease,
            box-shadow 0.2s ease,
            padding 0.2s ease;
    }

    .card.expanded {
        padding: 16px;
        box-shadow:
            0 14px 34px rgba(0, 0, 0, 0.2),
            inset 0 1px 0 rgba(255, 255, 255, 0.26);
        transform: translateY(-1px);
    }

    .card.has-error {
        background: linear-gradient(
            155deg,
            color-mix(in srgb, var(--card-bg) 72%, var(--bg-elevated) 28%) 0%,
            color-mix(in srgb, var(--card-bg) 72%, var(--accent-red, #d32f2f) 28%) 60%,
            color-mix(in srgb, var(--card-bg) 78%, var(--accent-red, #d32f2f) 22%) 100%
        );
        border-color: color-mix(in srgb, var(--accent-red, #d32f2f) 48%, var(--separator) 52%);
        box-shadow:
            0 12px 30px color-mix(in srgb, var(--accent-red, #d32f2f) 20%, transparent 80%),
            inset 0 1px 0 rgba(255, 255, 255, 0.2);
    }

    .card-header {
        margin-bottom: 10px;
    }

    .title-wrap {
        display: flex;
        align-items: center;
        justify-content: flex-start;
        width: 100%;
        gap: 10px;
    }

    .title-meta-wrap {
        display: flex;
        align-items: baseline;
        gap: 8px;
        min-width: 0;
    }

    .card-title {
        font-size: 15px;
        font-weight: 700;
        line-height: 1;
        color: var(--text-primary);
    }

    .card-header.has-error .card-title {
        color: var(--accent-red, #b71c1c);
        background: color-mix(in srgb, var(--accent-red, #d32f2f) 18%, white 82%);
        border: 1px solid color-mix(in srgb, var(--accent-red, #d32f2f) 42%, transparent 58%);
        border-radius: 8px;
        padding: 4px 8px;
    }

    .title-separator {
        font-size: 12px;
        line-height: 1;
        color: var(--text-secondary);
        opacity: 0.7;
    }

    .card-meta {
        font-size: 11px;
        font-weight: 600;
        line-height: 1;
        color: var(--text-secondary);
    }

    .card-meta.status-on {
        color: var(--accent-green, #4caf50);
    }

    .card-meta.status-off {
        color: var(--accent-red, #f44336);
    }

    .card-meta.status-sleep {
        color: var(--accent-yellow, #ff9800);
    }

    .warning-indicator {
        width: 20px;
        height: 20px;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        margin-left: auto;
        border-radius: 999px;
        color: var(--accent-green, #2e7d32);
        background: color-mix(in srgb, var(--accent-green, #2e7d32) 14%, white 86%);
        border: 1px solid color-mix(in srgb, var(--accent-green, #2e7d32) 40%, transparent 60%);
        flex: 0 0 auto;
    }

    .warning-indicator.is-error {
        color: var(--accent-red, #d32f2f);
        background: color-mix(in srgb, var(--accent-red, #d32f2f) 14%, white 86%);
        border-color: color-mix(in srgb, var(--accent-red, #d32f2f) 40%, transparent 60%);
    }

    .warning-indicator svg {
        width: 14px;
        height: 14px;
        fill: currentColor;
    }
</style>
