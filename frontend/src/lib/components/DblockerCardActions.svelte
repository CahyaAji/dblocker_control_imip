<script lang="ts">
    import { slide } from "svelte/transition";

    export let isExpanded = false;
    export let canReadLastState = false;
    export let showAdvancedActions = false;
    export let onReadLastState: () => void;
    export let onApply: () => void;
    export let onToggleAdvanced: () => void;
    export let onToggleExpanded: () => void;
</script>

<div class="action-row">
    {#if isExpanded}
        <div class="action-group" transition:slide={{ duration: 300 }}>
            <button
                class="reload-btn"
                type="button"
                disabled={!canReadLastState}
                on:click={onReadLastState}
            >
                Read Last State
            </button>
            <button class="apply-btn" type="button" on:click={onApply}>
                Apply
            </button>
            <button
                class="advance-btn"
                type="button"
                on:click={onToggleAdvanced}
                aria-expanded={showAdvancedActions}
            >
                {showAdvancedActions ? "Hide Advance" : "Advance"}
            </button>
            <button
                class="detail-btn"
                type="button"
                on:click={onToggleExpanded}
                aria-expanded={isExpanded}
            >
                Show Less
            </button>
        </div>
    {:else}
        <div class="collapsed-action" transition:slide={{ duration: 300 }}>
            <button
                class="detail-btn"
                type="button"
                on:click={onToggleExpanded}
                aria-expanded={isExpanded}
            >
                Show Details
            </button>
        </div>
    {/if}
</div>

<style>
    .action-row {
        margin-top: 10px;
        display: flex;
        flex-direction: column;
        gap: 8px;
        overflow: hidden;
    }

    .collapsed-action {
        width: 100%;
    }

    /* Removed unused .action-placeholder selector */

    .action-group {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 8px;
    }

    .detail-btn,
    .apply-btn,
    .reload-btn,
    .advance-btn {
        width: 100%;
        border-radius: 10px;
        padding: 8px 10px;
        font-size: 12px;
        font-weight: 700;
        cursor: pointer;
        transition:
            background 0.2s ease,
            border-color 0.2s ease,
            opacity 0.2s ease,
            transform 0.12s ease,
            box-shadow 0.12s ease;
    }

    .detail-btn,
    .reload-btn,
    .advance-btn {
        border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
        background: color-mix(
            in srgb,
            var(--card-bg) 78%,
            var(--bg-elevated) 22%
        );
        color: var(--text-primary);
    }

    .detail-btn:hover,
    .reload-btn:not(:disabled):hover,
    .advance-btn:hover {
        background: color-mix(
            in srgb,
            var(--card-bg) 68%,
            var(--accent-cyan) 32%
        );
        border-color: color-mix(
            in srgb,
            var(--accent-cyan) 45%,
            var(--separator)
        );
    }

    .detail-btn:active,
    .apply-btn:active,
    .reload-btn:not(:disabled):active,
    .advance-btn:active {
        transform: translateY(1px) scale(0.985);
        box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.22);
    }

    .apply-btn {
        border: 1px solid
            color-mix(in srgb, var(--accent-green) 45%, transparent);
        background: color-mix(
            in srgb,
            var(--accent-green) 20%,
            var(--card-bg) 80%
        );
        color: var(--text-primary);
    }

    .apply-btn:hover {
        background: color-mix(
            in srgb,
            var(--accent-green) 30%,
            var(--card-bg) 70%
        );
        border-color: color-mix(in srgb, var(--accent-green) 70%, transparent);
    }

    .reload-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
</style>
