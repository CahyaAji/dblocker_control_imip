<script lang="ts">
    import { onMount } from "svelte";
    import {
        bridgeStore,
        subscribeBridge,
        unsubscribeBridge,
    } from "../store/bridgeStore";

    $: entries = Object.entries($bridgeStore)
        .filter(([topic]) => topic.endsWith("/sta"))
        .sort(([a], [b]) => a.localeCompare(b));

    onMount(() => {
        subscribeBridge();
        return () => unsubscribeBridge();
    });
</script>

<div class="status-box">
    <h4>Status Topic (/sta)</h4>
    {#if entries.length === 0}
        <p>No status updates yet.</p>
    {:else}
        {#each entries as [topic, payload]}
            <div class="status-row">
                <div class="topic">@{topic}</div>
                <div class="payload">{payload}</div>
            </div>
        {/each}
    {/if}
</div>

<style>
    .status-box {
        width: 100%;
        padding: 6px;
        border-radius: 10px;
        border: 1px solid color-mix(in srgb, var(--separator) 72%, transparent);
        background: color-mix(in srgb, var(--card-bg) 88%, var(--bg-elevated) 12%);
        color: var(--text-primary);
        font-size: 0.82rem;
    }

    h4 {
        margin: 0 0 6px 0;
        font-size: 0.82rem;
        font-weight: 700;
    }

    p {
        margin: 0;
    }

    .status-row {
        padding: 5px;
        border-radius: 8px;
        border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
        margin-top: 5px;
        background: color-mix(in srgb, var(--bg-elevated) 55%, transparent);
    }

    .topic {
        font-weight: 600;
        margin-bottom: 2px;
        word-break: break-word;
    }

    .payload {
        word-break: break-word;
        white-space: pre-wrap;
    }
</style>
