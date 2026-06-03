<script lang="ts">
    import { onMount } from "svelte";
    import {
        authStore,
        authFetch,
        verifyToken,
        logout,
    } from "../store/authStore";
    import { API_BASE } from "../utils/api";
    import DroneDetectionToast from "./DroneDetectionToast.svelte";
    import LoginPage from "./LoginPage.svelte";

    interface WhitelistEntry {
        id: number;
        type: "unique_id" | "target_name";
        value: string;
        note: string;
        created_at: string;
    }

    let entries = $state<WhitelistEntry[]>([]);
    let loading = $state(false);
    let authorized = $state(false);
    let darkMode = $state(true);

    // Add-form state
    let formType = $state<"unique_id" | "target_name">("unique_id");
    let formValue = $state("");
    let formNote = $state("");
    let formError = $state("");
    let formSubmitting = $state(false);

    function loadTheme() {
        try {
            const stored = localStorage.getItem("app-settings");
            if (stored) {
                const parsed = JSON.parse(stored);
                darkMode = parsed.theme === "dark";
            }
        } catch {
            /* */
        }
        applyTheme();
    }

    function applyTheme() {
        if (typeof document !== "undefined") {
            document.documentElement.classList.toggle("dark", darkMode);
        }
    }

    function toggleTheme() {
        darkMode = !darkMode;
        applyTheme();
        try {
            const stored = localStorage.getItem("app-settings");
            const settings = stored ? JSON.parse(stored) : {};
            settings.theme = darkMode ? "dark" : "light";
            localStorage.setItem("app-settings", JSON.stringify(settings));
        } catch {
            /* */
        }
    }

    onMount(() => {
        loadTheme();
        if ($authStore.token) {
            verifyToken().then((valid) => {
                if (valid) {
                    authorized = true;
                    fetchWhitelist();
                }
            });
        }
    });

    async function fetchWhitelist() {
        loading = true;
        try {
            const res = await authFetch(`${API_BASE}/api/whitelist`);
            if (res.ok) {
                const json = await res.json();
                entries = json.data || [];
            }
        } catch {
            console.error("Failed to fetch whitelist");
        } finally {
            loading = false;
        }
    }

    async function addEntry() {
        formError = "";
        const v = formValue.trim();
        if (!v) {
            formError = "Value is required.";
            return;
        }
        formSubmitting = true;
        try {
            const res = await authFetch(`${API_BASE}/api/whitelist`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    type: formType,
                    value: v,
                    note: formNote.trim(),
                }),
            });
            if (res.ok) {
                formValue = "";
                formNote = "";
                await fetchWhitelist();
            } else {
                const json = await res.json().catch(() => ({}));
                formError = json.error || `Error ${res.status}`;
            }
        } catch (e) {
            formError = "Network error";
        } finally {
            formSubmitting = false;
        }
    }

    async function deleteEntry(id: number) {
        try {
            const res = await authFetch(`${API_BASE}/api/whitelist/${id}`, {
                method: "DELETE",
            });
            if (res.ok) {
                await fetchWhitelist();
            }
        } catch {
            console.error("Failed to delete whitelist entry");
        }
    }

    function formatTime(ts: string): string {
        const d = new Date(ts);
        return d.toLocaleString("en-GB", {
            year: "numeric",
            month: "short",
            day: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
        });
    }
</script>

<DroneDetectionToast />

{#if !$authStore.token}
    <LoginPage />
{:else if !authorized}
    <div class="page">
        <div class="access-denied"><h2>Loading...</h2></div>
    </div>
{:else}
    <div class="page">
        <header class="page-header">
            <div class="header-left">
                <a href="/dashboard" class="back-link">← Dashboard</a>
                <h1>Drone Whitelist</h1>
                <span class="entry-count">{entries.length} entries</span>
            </div>
            <div class="header-right">
                <button
                    class="btn-theme"
                    onclick={toggleTheme}
                    title="Toggle theme"
                >
                    {#if darkMode}☀{:else}🌙{/if}
                </button>
                <button class="btn-logout" onclick={logout} title="Logout">
                    Logout
                </button>
            </div>
        </header>

        <div class="content">
            <!-- Add form -->
            <section class="add-card">
                <h2>Add Whitelist Entry</h2>
                <p class="card-desc">
                    Whitelisted drones are still logged but will <strong
                        >never</strong
                    > trigger dblocker activation.
                </p>
                <form
                    class="add-form"
                    onsubmit={(e) => {
                        e.preventDefault();
                        addEntry();
                    }}
                >
                    <div class="form-row">
                        <label class="form-label" for="type">Type</label>
                        <select
                            id="type"
                            class="form-select"
                            bind:value={formType}
                        >
                            <option value="unique_id">Unique ID</option>
                            <option value="target_name">Target Name</option>
                        </select>
                    </div>
                    <div class="form-row">
                        <label class="form-label" for="value">Value</label>
                        <input
                            id="value"
                            class="form-input"
                            type="text"
                            placeholder={formType === "unique_id"
                                ? "e.g. AABBCCDD"
                                : "e.g. DJI Mini 3"}
                            bind:value={formValue}
                        />
                    </div>
                    <div class="form-row">
                        <label class="form-label" for="note">Note</label>
                        <input
                            id="note"
                            class="form-input"
                            type="text"
                            placeholder="Optional description"
                            bind:value={formNote}
                        />
                    </div>
                    {#if formError}
                        <p class="form-error">{formError}</p>
                    {/if}
                    <button
                        type="submit"
                        class="btn-add"
                        disabled={formSubmitting}
                    >
                        {formSubmitting ? "Adding..." : "Add to Whitelist"}
                    </button>
                </form>
            </section>

            <!-- Table -->
            <section class="table-card">
                <h2>Whitelisted Drones</h2>
                {#if loading}
                    <div class="loading-text">Loading...</div>
                {:else if entries.length === 0}
                    <div class="empty-text">No whitelist entries yet.</div>
                {:else}
                    <div class="table-wrap">
                        <table class="wl-table">
                            <thead>
                                <tr>
                                    <th>Type</th>
                                    <th>Value</th>
                                    <th>Note</th>
                                    <th>Added</th>
                                    <th></th>
                                </tr>
                            </thead>
                            <tbody>
                                {#each entries as entry (entry.id)}
                                    <tr>
                                        <td>
                                            <span
                                                class="type-badge type-{entry.type}"
                                            >
                                                {entry.type === "unique_id"
                                                    ? "Unique ID"
                                                    : "Target Name"}
                                            </span>
                                        </td>
                                        <td class="val-cell">{entry.value}</td>
                                        <td class="note-cell"
                                            >{entry.note || "—"}</td
                                        >
                                        <td class="time-cell"
                                            >{formatTime(entry.created_at)}</td
                                        >
                                        <td class="action-cell">
                                            <button
                                                class="btn-delete"
                                                onclick={() =>
                                                    deleteEntry(entry.id)}
                                                title="Remove from whitelist"
                                            >
                                                Remove
                                            </button>
                                        </td>
                                    </tr>
                                {/each}
                            </tbody>
                        </table>
                    </div>
                {/if}
            </section>
        </div>
    </div>
{/if}

<style>
    .page {
        max-width: 900px;
        margin: 0 auto;
        padding: 24px 16px;
    }

    .access-denied {
        text-align: center;
        padding: 60px 20px;
        color: var(--text-secondary);
    }

    .page-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 20px;
        flex-wrap: wrap;
        gap: 12px;
    }

    .header-left {
        display: flex;
        align-items: center;
        gap: 16px;
    }

    .header-right {
        display: flex;
        align-items: center;
        gap: 10px;
    }

    .back-link {
        color: var(--accent-cyan);
        text-decoration: none;
        font-size: 13px;
        font-weight: 600;
    }

    .back-link:hover {
        text-decoration: underline;
    }

    h1 {
        font-size: 20px;
        font-weight: 700;
        color: var(--text-primary);
        margin: 0;
    }

    .entry-count {
        font-size: 12px;
        color: var(--text-secondary);
        font-weight: 500;
    }

    .btn-theme {
        padding: 4px 10px;
        border: 1px solid var(--separator);
        border-radius: 8px;
        background: var(--card-bg);
        cursor: pointer;
        font-size: 16px;
        line-height: 1;
    }

    .btn-theme:hover {
        border-color: var(--accent-cyan);
    }

    .btn-logout {
        padding: 5px 12px;
        border: 1px solid var(--separator);
        border-radius: 8px;
        background: var(--card-bg);
        color: var(--text-secondary);
        font-size: 12px;
        font-weight: 600;
        cursor: pointer;
    }

    .btn-logout:hover {
        color: var(--text-primary);
        border-color: var(--text-secondary);
    }

    .content {
        display: flex;
        flex-direction: column;
        gap: 14px;
    }

    .add-card,
    .table-card {
        background: var(--card-bg);
        border: 1px solid var(--separator);
        border-radius: 14px;
        padding: 18px 20px;
    }

    h2 {
        margin: 0 0 4px;
        font-size: 14px;
        font-weight: 700;
        color: var(--text-primary);
    }

    .card-desc {
        margin: 0 0 14px;
        font-size: 12px;
        color: var(--text-secondary);
    }

    .add-form {
        display: flex;
        flex-direction: column;
        gap: 10px;
    }

    .form-row {
        display: flex;
        align-items: center;
        gap: 10px;
    }

    .form-label {
        width: 72px;
        flex-shrink: 0;
        font-size: 12px;
        font-weight: 600;
        color: var(--text-secondary);
    }

    .form-input,
    .form-select {
        flex: 1;
        background: var(--bg-color);
        border: 1px solid var(--separator);
        color: var(--text-primary);
        padding: 6px 10px;
        border-radius: 8px;
        font-size: 13px;
        font-family: inherit;
        outline: none;
    }

    .form-input:focus,
    .form-select:focus {
        border-color: var(--accent-cyan);
    }

    .form-error {
        margin: 0;
        font-size: 12px;
        color: #ff4444;
    }

    .btn-add {
        align-self: flex-start;
        background: var(--accent-blue);
        color: #fff;
        border: none;
        padding: 6px 16px;
        border-radius: 8px;
        cursor: pointer;
        font-size: 13px;
        font-weight: 600;
        transition: opacity 0.15s;
    }

    .btn-add:disabled {
        opacity: 0.45;
        cursor: default;
    }

    .btn-add:not(:disabled):hover {
        opacity: 0.85;
    }

    .table-wrap {
        overflow-x: auto;
    }

    .wl-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 12px;
    }

    .wl-table thead {
        position: sticky;
        top: 0;
        z-index: 1;
    }

    .wl-table th {
        text-align: left;
        padding: 10px 12px;
        font-size: 11px;
        font-weight: 700;
        color: var(--text-secondary);
        text-transform: uppercase;
        letter-spacing: 0.3px;
        border-bottom: 1px solid var(--separator);
        background: var(--card-bg);
        white-space: nowrap;
    }

    .wl-table td {
        padding: 9px 12px;
        border-bottom: 1px solid
            color-mix(in srgb, var(--separator) 50%, transparent);
        color: var(--text-primary);
        white-space: nowrap;
        vertical-align: middle;
    }

    .wl-table tbody tr:last-child td {
        border-bottom: none;
    }

    .wl-table tbody tr:hover td {
        background: color-mix(in srgb, var(--accent-cyan) 5%, transparent);
    }

    .type-badge {
        display: inline-block;
        padding: 2px 8px;
        border-radius: 6px;
        font-size: 11px;
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: 0.3px;
    }

    .type-unique_id {
        background: color-mix(in srgb, var(--accent-blue) 15%, var(--card-bg));
        color: var(--accent-blue);
    }

    .type-target_name {
        background: color-mix(in srgb, var(--accent-green) 15%, var(--card-bg));
        color: var(--accent-green);
    }

    .val-cell {
        font-family: monospace;
        font-variant-numeric: tabular-nums;
        font-size: 12px;
    }

    .note-cell {
        color: var(--text-secondary);
        font-size: 12px;
    }

    .time-cell {
        color: var(--text-secondary);
        font-size: 11px;
        font-variant-numeric: tabular-nums;
    }

    .action-cell {
        text-align: right;
    }

    .btn-delete {
        background: color-mix(in srgb, #ff4444 12%, var(--card-bg));
        color: #ff4444;
        border: 1px solid color-mix(in srgb, #ff4444 30%, transparent);
        padding: 3px 10px;
        border-radius: 6px;
        cursor: pointer;
        font-size: 11px;
        font-weight: 600;
        transition: background 0.15s;
    }

    .btn-delete:hover {
        background: color-mix(in srgb, #ff4444 25%, var(--card-bg));
    }

    .empty-text,
    .loading-text {
        text-align: center;
        padding: 40px 20px;
        color: var(--text-secondary);
        font-size: 13px;
    }
</style>
