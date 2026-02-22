<script lang="ts">
    import { onMount } from "svelte";
    import { API_BASE } from "../utils/api";

    let messages: string[] = [];
    let status = "connecting…";
    let source: EventSource | null = null;

    onMount(() => {
        source = new EventSource(`${API_BASE}events`);
        source.onopen = () => (status = "connected");
        source.onerror = () => (status = "error (will retry)");
        source.onmessage = (ev) => (messages = [...messages, ev.data]);

        return () => {
            source?.close();
            source = null;
        };
    });
</script>

<h2>Live events</h2>
<p>Status: {status}</p>

{#if messages.length === 0}
    <p>No messages yet.</p>
{:else}
    <ul>
        {#each messages as msg, i}
            <li><strong>{i + 1}.</strong> {msg}</li>
        {/each}
    </ul>
{/if}