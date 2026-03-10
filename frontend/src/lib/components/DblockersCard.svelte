<script lang="ts">
    import { onMount } from "svelte";
    import DblockerCardItem from "./DblockerCardItem.svelte";
    import {
        dblockerStore,
        fetchDBlockers,
        type DBlocker,
    } from "../store/dblockerStore";

    let dblockers: DBlocker[] = [];

    onMount(async () => {
        await fetchDBlockers();
    });

    $: dblockers = [...$dblockerStore].sort((a, b) => a.id - b.id);
</script>

<div class="list">
    {#each dblockers as dblocker (dblocker.id)}
        <DblockerCardItem {dblocker} />
    {:else}
        <div class="empty">No DBlockers found</div>
    {/each}
</div>

<style>
    .list {
        display: flex;
        flex-direction: column;
        gap: 12px;
        padding: 10px 6px;
        flex: 1;
        overflow-y: auto;
        scrollbar-color: var(--separator) var(--bg-color);
        min-height: 0;
    }

    .empty {
        padding: 18px;
        border-radius: 14px;
        border: 1px dashed var(--separator);
        color: var(--text-secondary);
        text-align: center;
        background: color-mix(
            in srgb,
            var(--card-bg) 88%,
            var(--bg-elevated) 12%
        );
    }
</style>
