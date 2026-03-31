<script>
    import DblockersCard from "./DblockersCard.svelte";
    import DBlockersList from "./DBlockersList.svelte";
    import ScheduleManager from "./ScheduleManager.svelte";

    let activeTab = $state("dblocker");
</script>

<div class="container">
    <div class="tabs">
        <button
            class:active={activeTab === "dblocker"}
            onclick={() => (activeTab = "dblocker")}>DBlocker</button
        >
        <button
            class:active={activeTab === "settings"}
            onclick={() => (activeTab = "settings")}>Auto Mode</button
        >

        <button
            class:active={activeTab === "scheduler"}
            onclick={() => (activeTab = "scheduler")}>Scheduler</button
        >
    </div>

    {#if activeTab === "settings"}
        <div class="other" style="background-color: lightblue;">
            Content for Tab 1
        </div>
    {:else if activeTab === "dblocker"}
        <DblockersCard />
    {:else if activeTab === "scheduler"}
        <ScheduleManager />
    {/if}
</div>

<style>
    .container {
        display: flex;
        flex-direction: column;
        height: 100%;
        overflow: hidden;
        gap: 0px;
    }

    .other {
        display: flex;
        min-height: 200px;
        width: 100%;
        background: linear-gradient(
            160deg,
            color-mix(in srgb, var(--card-bg) 90%, var(--accent-cyan) 10%) 0%,
            color-mix(in srgb, var(--card-bg) 92%, var(--accent-blue) 8%) 100%
        );
        margin-bottom: 4px;
        justify-items: center;
        align-items: center;
        border: 1px solid color-mix(in srgb, var(--separator) 72%, transparent);
        border-radius: 14px;
        justify-content: center;
        color: var(--text-secondary);
        font-weight: 600;
    }

    .tabs {
        display: flex;
        gap: 6px;
        padding: 4px;
        background: color-mix(
            in srgb,
            var(--card-bg) 88%,
            var(--bg-elevated) 12%
        );
        border: 1px solid color-mix(in srgb, var(--separator) 75%, transparent);
        border-radius: 999px;
    }

    button {
        padding: 7px 12px;
        cursor: pointer;
        background: none;
        border: none;
        font-size: 12px;
        font-weight: 700;
        border-radius: 999px;
        color: var(--text-secondary);
        transition: all 0.2s ease;
        white-space: nowrap;
    }

    button:hover {
        color: var(--text-primary);
        background: color-mix(
            in srgb,
            var(--card-bg) 84%,
            var(--bg-elevated) 16%
        );
    }

    button.active {
        background: var(--card-bg);
        box-shadow: 0 8px 16px rgba(18, 35, 48, 0.12);
        color: var(--text-primary);
    }
</style>
