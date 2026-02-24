<script lang="ts">
    import { onMount } from "svelte";
    import { API_BASE } from "../utils/api";

    type BridgeEvent = {
        topic?: string;
        payload?: string;
    };

    type ParsedPayload = {
        currents: number[];
        temperatures: number[];
        digital: boolean | null;
        raw: string;
    };

    let latestByTopic: Record<string, string> = {};
    let status = "connecting…";
    let source: EventSource | null = null;

    const parsePayload = (payload: string): ParsedPayload | null => {
        const [numericPart, digitalPart] = payload.split("|");
        if (!numericPart) return null;

        const values = numericPart
            .split(",")
            .map((item) => Number(item.trim()))
            .filter((item) => !Number.isNaN(item));

        if (values.length < 20) return null;

        const digitalRaw = (digitalPart ?? "").trim();
        const digital =
            digitalRaw === "1" ? true : digitalRaw === "0" ? false : null;

        return {
            currents: values.slice(0, 18),
            temperatures: values.slice(18, 20),
            digital,
            raw: payload,
        };
    };

    const handleMessage = (ev: MessageEvent<string>) => {
        try {
            const data: BridgeEvent = JSON.parse(ev.data);
            const topic = data.topic?.trim();
            const payload = data.payload ?? "";

            if (!topic) return;

            latestByTopic = {
                ...latestByTopic,
                [topic]: payload,
            };
        } catch (err) {
            console.error("Failed to parse event data:", err);
            // Ignore malformed event payloads.
        }
    };

    $: topicEntries = Object.entries(latestByTopic).sort(([a], [b]) =>
        a.localeCompare(b),
    );
    $: parsedEntries = topicEntries.map(([topic, payload]) => ({
        topic,
        parsed: parsePayload(payload),
    }));

    onMount(() => {
        source = new EventSource(`${API_BASE}/events`);
        source.onopen = () => (status = "connected");
        source.onerror = () => (status = "error (will retry)");
        source.onmessage = handleMessage;

        return () => {
            source?.close();
            source = null;
        };
    });
</script>

<div class="receiver-box">
    <h4>Development Only</h4>
    <p>Status: {status}</p>

    {#if parsedEntries.length === 0}
        <p>No topic updates yet.</p>
    {:else}
        {#each parsedEntries as entry}
            <div class="topic-section">
                <h4>@{entry.topic}</h4>

                {#if entry.parsed}
                    <table>
                        <thead>
                            <tr>
                                <th>Field</th>
                                <th>Value</th>
                            </tr>
                        </thead>
                        <tbody>
                            {#each entry.parsed.currents as value, idx}
                                <tr>
                                    <td>Current {idx + 1}</td>
                                    <td>{value}</td>
                                </tr>
                            {/each}

                            {#each entry.parsed.temperatures as value, idx}
                                <tr>
                                    <td>Temperature {idx + 1}</td>
                                    <td>{value / 100}</td>
                                </tr>
                            {/each}

                            <tr>
                                <td>Slave Connected</td>
                                <td>
                                    {#if entry.parsed.digital === null}
                                        -
                                    {:else if entry.parsed.digital}
                                        true
                                    {:else}
                                        false
                                    {/if}
                                </td>
                            </tr>
                        </tbody>
                    </table>
                {:else}
                    <p>Invalid payload format: {latestByTopic[entry.topic]}</p>
                {/if}
            </div>
        {/each}
    {/if}
</div>

<style>
    .receiver-box {
        width: 100%;
        padding: 8px;
        font-size: 0.9rem;
        overflow: auto;
        color: black;
    }

    .topic-section {
        margin-top: 10px;
        border: 1px solid #ddd;
        border-radius: 6px;
        padding: 8px;
    }

    h4 {
        margin: 0 0 8px;
    }

    table {
        width: 100%;
        border-collapse: collapse;
    }

    th,
    td {
        border: 1px solid #ddd;
        padding: 4px 6px;
        text-align: left;
        word-break: break-word;
    }

    th {
        background: #f5f5f5;
    }
</style>