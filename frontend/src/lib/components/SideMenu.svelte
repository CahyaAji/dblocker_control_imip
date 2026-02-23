<script>
    import DBlockersList from "./DBlockersList.svelte";

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
        <DBlockersList />
    {:else if activeTab === "scheduler"}
        <div class="other" style="background-color: orange;">
            Content for Tab 3
        </div>
    {/if}
</div>

<style>
    .container {
        display: flex;
        flex-direction: column;
        height: 100%;
        overflow: hidden;
    }

    .other {
        display: flex;
        height: 200px;
        width: 100%;
        background-color: gold;
        margin-bottom: 4px;
        justify-items: center;
        align-items: center;
        border-bottom: 2px solid #333;
        justify-content: center;
    }

    .tabs {
        display: flex;
        border-bottom: 1px solid var(--separator);
        margin-bottom: 2px;
        
    }

    button {
        padding: 4px 12px;
        cursor: pointer;
        background: none;
        border: none;
        font-size: 13pt;
        border-bottom: 3px solid transparent;
        border-right: 1px solid var(--separator);
        color: var(--text-secondary);
    }

    button.active {
        border-bottom-color: var(--text-primary);
        font-weight: bold;
        color: var(--text-primary);
    }
</style>
