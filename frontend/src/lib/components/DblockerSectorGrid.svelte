<script lang="ts">
    import { slide } from "svelte/transition";
    import type { DBlockerConfig } from "../store/dblockerStore";

    export let isExpanded = false;
    export let showAdvancedActions = false;
    export let liveConfig: DBlockerConfig[] = [];
    export let editableConfig: DBlockerConfig[] = [];

    export let onToggleSignal: (
        sectorIndex: number,
        key: keyof DBlockerConfig,
    ) => void;
    export let onAdvancedAction: (action: "sleep" | "reboot") => void;
</script>

<div class="content-wrap" class:expanded={isExpanded}>
    {#if isExpanded}
        <div class="expanded-mode" transition:slide={{ duration: 300 }}>
            <div class="expanded-columns">
                <div class="expanded-col">
                    {#each liveConfig.slice(0, 3) as config, index}
                        {@const sectorCfg = editableConfig[index] || config}
                        <div class="sector interactive">
                            <div class="sector-title">Sector {index + 1}</div>
                            <div class="control-row">
                                <div class="control-label">Block RC</div>
                                <label class="switch">
                                    <input
                                        type="checkbox"
                                        checked={sectorCfg.signal_ctrl}
                                        on:change={() =>
                                            onToggleSignal(
                                                index,
                                                "signal_ctrl",
                                            )}
                                    />
                                    <span class="slider"></span>
                                </label>
                            </div>
                            <div class="control-row">
                                <div class="control-label">Block GPS</div>
                                <label class="switch">
                                    <input
                                        type="checkbox"
                                        checked={sectorCfg.signal_gps}
                                        on:change={() =>
                                            onToggleSignal(index, "signal_gps")}
                                    />
                                    <span class="slider"></span>
                                </label>
                            </div>
                        </div>
                    {/each}
                </div>

                <div class="expanded-col">
                    {#each liveConfig.slice(3, 6) as config, index}
                        {@const sectorIndex = index + 3}
                        {@const sectorCfg =
                            editableConfig[sectorIndex] || config}
                        <div class="sector interactive">
                            <div class="sector-title">
                                Sector {sectorIndex + 1}
                            </div>
                            <div class="control-row">
                                <div class="control-label">Block RC</div>
                                <label class="switch">
                                    <input
                                        type="checkbox"
                                        checked={sectorCfg.signal_ctrl}
                                        on:change={() =>
                                            onToggleSignal(
                                                sectorIndex,
                                                "signal_ctrl",
                                            )}
                                    />
                                    <span class="slider"></span>
                                </label>
                            </div>
                            <div class="control-row">
                                <div class="control-label">Block GPS</div>
                                <label class="switch">
                                    <input
                                        type="checkbox"
                                        checked={sectorCfg.signal_gps}
                                        on:change={() =>
                                            onToggleSignal(
                                                sectorIndex,
                                                "signal_gps",
                                            )}
                                    />
                                    <span class="slider"></span>
                                </label>
                            </div>
                        </div>
                    {/each}
                </div>
            </div>

            {#if showAdvancedActions}
                <div
                    class="advanced-sector-wrap"
                    transition:slide={{ duration: 300 }}
                >
                    <div class="sector interactive advanced-sector">
                        <div class="sector-title">Advanced Action</div>
                        <button
                            class="advanced-action-btn sleep"
                            type="button"
                            on:click={() => onAdvancedAction("sleep")}
                        >
                            Sleep DBlocker
                        </button>
                        <button
                            class="advanced-action-btn reboot"
                            type="button"
                            on:click={() => onAdvancedAction("reboot")}
                        >
                            Reboot DBlocker
                        </button>
                    </div>
                </div>
            {/if}
        </div>
    {:else}
        <div class="compact-grid" transition:slide={{ duration: 300 }}>
            {#each liveConfig as config, index}
                <div class="sector">
                    <div class="sector-index">{index + 1}</div>
                    <div
                        class="notif"
                        class:on={config.signal_ctrl}
                        title={config.signal_ctrl ? "CTRL ON" : "CTRL OFF"}
                    ></div>
                    <div
                        class="notif"
                        class:on={config.signal_gps}
                        title={config.signal_gps ? "GPS ON" : "GPS OFF"}
                    ></div>
                </div>
            {/each}
        </div>
    {/if}
</div>

<style>
    .content-wrap {
        transition:
            min-height 0.32s cubic-bezier(0.4, 0.2, 0.2, 1),
            max-height 0.32s cubic-bezier(0.4, 0.2, 0.2, 1),
            padding 0.28s cubic-bezier(0.4, 0.2, 0.2, 1);
        overflow: hidden;
    }

    .content-wrap.expanded {
        overflow: visible;
    }
    .compact-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(38px, 1fr));
        gap: 8px;
    }

    .expanded-columns {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 10px;
    }

    .expanded-col {
        display: flex;
        flex-direction: column;
        gap: 10px;
    }

    .sector {
        display: flex;
        flex-direction: column;
        gap: 6px;
        min-width: 38px;
        padding: 8px 6px;
        background-color: color-mix(
            in srgb,
            var(--card-bg) 85%,
            var(--bg-elevated) 15%
        );
        border-radius: 12px;
        border: 1px solid color-mix(in srgb, var(--separator) 68%, transparent);
        align-items: center;
        box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.3);
    }

    .sector.interactive {
        gap: 2px;
        align-items: stretch;
        padding: 10px;
        border-radius: 10px;
        background-color: color-mix(
            in srgb,
            var(--card-bg) 78%,
            var(--bg-elevated) 22%
        );
        border-color: color-mix(in srgb, var(--separator) 75%, transparent);
        box-shadow: none;
    }

    .sector-title {
        font-size: 11px;
        font-weight: 700;
        color: var(--text-primary);
        letter-spacing: 0.02em;
        padding-bottom: 4px;
        border-bottom: 1px solid
            color-mix(in srgb, var(--separator) 70%, transparent);
        margin-bottom: 2px;
    }

    .control-row {
        width: 100%;
        min-height: 26px;
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 8px;
    }

    .control-label {
        font-size: 11px;
        font-weight: 700;
        color: var(--text-secondary);
        letter-spacing: 0.02em;
    }

    .sector :global(.switch) {
        transform: scale(0.92);
        transform-origin: right center;
    }

    .sector-index {
        font-size: 11px;
        font-weight: 700;
        color: var(--text-secondary);
    }

    .notif {
        width: 11px;
        height: 11px;
        border-radius: 50%;
        background: radial-gradient(circle at 35% 35%, #3a3a3a 0%, #1f1f1f 70%);
        border: 1px solid #0f0f0f;
        box-shadow:
            inset 0 1px 1px rgba(255, 255, 255, 0.12),
            inset 0 -1px 2px rgba(0, 0, 0, 0.6);
        transition:
            background 0.2s ease,
            box-shadow 0.2s ease,
            border-color 0.2s ease;
    }

    .notif.on {
        background: radial-gradient(
            circle at 35% 35%,
            #9dff8d 0%,
            #39d353 45%,
            #1f9f39 100%
        );
        border-color: #2ec84d;
        box-shadow:
            0 0 5px rgba(57, 211, 83, 0.9),
            0 0 10px rgba(57, 211, 83, 0.7),
            inset 0 1px 1px rgba(255, 255, 255, 0.35);
    }

    .advanced-sector-wrap {
        margin-top: 10px;
    }

    .advanced-sector {
        align-items: stretch;
        gap: 8px;
    }

    .advanced-action-btn {
        border: 1px solid color-mix(in srgb, var(--separator) 72%, transparent);
        background: color-mix(
            in srgb,
            var(--card-bg) 75%,
            var(--bg-elevated) 25%
        );
        color: var(--text-primary);
        border-radius: 10px;
        padding: 8px 10px;
        font-size: 12px;
        font-weight: 700;
        cursor: pointer;
        transition:
            background 0.2s ease,
            border-color 0.2s ease,
            transform 0.12s ease,
            box-shadow 0.12s ease;
    }

    .advanced-action-btn:active {
        transform: translateY(1px) scale(0.985);
        box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.22);
    }

    .advanced-action-btn.sleep:hover {
        background: color-mix(in srgb, #f59e0b 20%, var(--card-bg) 80%);
        border-color: color-mix(in srgb, #f59e0b 45%, transparent);
    }

    .advanced-action-btn.reboot:hover {
        background: color-mix(in srgb, #ef4444 20%, var(--card-bg) 80%);
        border-color: color-mix(in srgb, #ef4444 45%, transparent);
    }
</style>
