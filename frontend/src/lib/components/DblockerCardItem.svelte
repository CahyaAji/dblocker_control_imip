<script lang="ts">
    import DblockerCardActions from "./DblockerCardActions.svelte";
    import DblockerSectorGrid from "./DblockerSectorGrid.svelte";
    import type { DBlocker, DBlockerConfig } from "../store/dblockerStore";
    import { updateDBlockerConfig } from "../store/dblockerStore";

    export let dblocker: DBlocker;

    let isExpanded = false;
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
        isExpanded = !isExpanded;
        if (isExpanded) {
            editableConfig = liveConfig.map((c) => ({ ...c }));
            hasEdited = false;
        } else {
            showAdvancedActions = false;
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
            // Wait for backend/store to match our local config
            const checkMatch = () => {
                const isMatch =
                    JSON.stringify(dblocker.config) ===
                    JSON.stringify(editableConfig);
                if (isMatch) {
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

    function handleAdvancedAction(action: "sleep" | "reboot") {
        console.log("[DBlockerCard] advanced action", {
            id: dblocker.id,
            action,
        });
    }
</script>

<div class="card" class:expanded={isExpanded}>
    <div class="card-header">
        <div class="title-wrap">
            <div class="card-title">{dblocker.name}</div>
            <div class="card-meta">ini diisi online/offline</div>
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
        onReadLastState={reloadFromLatest}
        onApply={applyConfig}
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

    .card-header {
        margin-bottom: 10px;
    }

    .title-wrap {
        display: flex;
        align-items: center;
        justify-content: space-between;
        width: 100%;
        gap: 8px;
    }

    .card-title {
        font-size: 15px;
        font-weight: 700;
        color: var(--text-primary);
    }

    .card-meta {
        font-size: 11px;
        font-weight: 600;
        color: var(--text-secondary);
    }

    .card-meta.online {
        color: var(--accent-green);
    }
</style>
