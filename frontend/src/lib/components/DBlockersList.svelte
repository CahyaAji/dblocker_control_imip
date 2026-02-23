<script lang="ts">
    import { onMount } from "svelte";
    import { API_BASE } from "../utils/api";
    import {
        dblockerStore,
        fetchDBlockers,
        type DBlocker,
        type DBlockerConfig,
    } from "../store/dblockerStore";

    // Track which cards are being edited; keep local copy of dblockers for UI editing.
    let editingIds: number[] = [];
    let dblockers: DBlocker[] = [];
    let debounceTimer: ReturnType<typeof setTimeout> | undefined;

    function toggleEditMode(blocker: DBlocker) {
        if (editingIds.includes(blocker.id)) {
            // console.log(blocker.id, $state.snapshot(blocker).config);
            updateDBlocker(blocker.id, blocker.config);
            editingIds = editingIds.filter((i) => i !== blocker.id);
        } else {
            editingIds = [...editingIds, blocker.id];
        }
    }

    async function updateDBlocker(blockerid: number, config: DBlockerConfig[]) {
        const payload = { id: blockerid, config };
        try {
            const res = await fetch(`${API_BASE}/api/dblockers/config`, {
                method: "PUT",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify(payload),
            });
            if (!res.ok) throw new Error("Update dblocker failed");
            return true;
        } catch (error) {
            console.error("Error updating dblocker:", error);
            return false;
        }
    }

    // React to store updates when not editing locally.
    $: if (editingIds.length === 0) {
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(() => {
            const sortedStoreData = [...$dblockerStore].sort(
                (a, b) => a.id - b.id,
            );
            if (JSON.stringify(dblockers) !== JSON.stringify(sortedStoreData)) {
                dblockers = sortedStoreData;
            }
        }, 300);
    }

    onMount(async () => {
        await fetchDBlockers();
        dblockers = [...$dblockerStore].sort((a, b) => a.id - b.id);
    });
</script>

<div class="list">
    {#each dblockers as blocker (blocker.id)}
        {@const isEditMode = editingIds.includes(blocker.id)}
        {@const cfg = blocker.config ?? []}
        <div class="card">
            <div class="card-header">
                <div>{blocker.name}</div>
                {#if isEditMode}
                    <button
                        class="btn-edit"
                        onclick={() => toggleEditMode(blocker)}>Apply</button
                    >
                {:else}
                    <button
                        class="btn-edit"
                        onclick={() => toggleEditMode(blocker)}>Edit</button
                    >
                {/if}
            </div>
            <div class="card-content">
                <div class="col">
                    {#each cfg.slice(0, 3) as config, index}
                        <div class="sector">
                            <div class="section-title">Sector {index + 1}</div>
                            <div class="control-group">
                                <div class="control-row">
                                    <div class="control-label">Blcker RC</div>
                                    <label class="switch">
                                        <input
                                            type="checkbox"
                                            bind:checked={config.signal_ctrl}
                                            disabled={!isEditMode}
                                        />
                                        <span class="slider"></span>
                                    </label>
                                </div>
                                <div class="control-row">
                                    <div class="control-label">Blcker GPS</div>
                                    <label class="switch">
                                        <input
                                            type="checkbox"
                                            bind:checked={config.signal_gps}
                                            disabled={!isEditMode}
                                        />
                                        <span class="slider"></span>
                                    </label>
                                </div>
                            </div>
                        </div>
                    {/each}
                </div>
                <div class="col">
                    {#each cfg.slice(3, 6) as config, index}
                        <div class="sector">
                            <div class="section-title">Sector {index + 4}</div>
                            <div class="control-group">
                                <div class="control-row">
                                    <div class="control-label">Blcker RC</div>
                                    <label class="switch">
                                        <input
                                            type="checkbox"
                                            bind:checked={config.signal_ctrl}
                                            disabled={!isEditMode}
                                        />
                                        <span class="slider"></span>
                                    </label>
                                </div>
                                <div class="control-row">
                                    <div class="control-label">Blcker GPS</div>
                                    <label class="switch">
                                        <input
                                            type="checkbox"
                                            bind:checked={config.signal_gps}
                                            disabled={!isEditMode}
                                        />
                                        <span class="slider"></span>
                                    </label>
                                </div>
                            </div>
                        </div>
                    {/each}
                </div>
            </div>
        </div>
    {:else}
        <div class="empty">No DBlockers found</div>
    {/each}
</div>

<style>
    .list {
        display: flex;
        flex-direction: column;
        overflow-y: auto;
        scrollbar-color: var(--separator) var(--bg-color);
        gap: 8px;
        flex: 1;
        min-height: 0;
        padding: 10px 6px;
    }
    .empty {
        text-align: center;
        color: #888;
        margin-top: 2rem;
    }
    .sector {
        display: flex;
        flex-direction: column;
        gap: 2px;
        margin-bottom: 4px;
    }
    .card-content {
        display: flex;
        gap: 10px;
    }
    .col {
        flex: 1;
        display: flex;
        flex-direction: column;
        gap: 6px;
    }
</style>
