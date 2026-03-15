<script lang="ts">
    import { onMount } from "svelte";
    import { API_BASE } from "../utils/api";

    type BridgeEvent = {
        topic?: string;
        payload?: string;
    };

    type ParsedPayload = {
        hallSensors: number[];
        temperature: number;
        slaveConnected: boolean;
        raw: string;
    };

    type ParsedStatusPayload = {
        mode: "OFF" | "SLEEP" | "ON";
        maskHex?: string;
        masterMask?: number;
        slaveMask?: number;
        raw: string;
    };

    let latestByTopic: Record<string, string> = {};
    let status = "connecting…";
    let source: EventSource | null = null;

    const parseReportPayload = (payload: string): ParsedPayload | null => {
        const [numericPart, slavePart] = payload.split("|");
        if (!numericPart) return null;

        const values = numericPart
            .split(",")
            .map((item) => Number(item.trim()))
            .filter((item) => !Number.isNaN(item));

        // MCU master format: hall[18],temperature|slaveConnected
        if (values.length !== 19) return null;

        const slaveRaw = (slavePart ?? "").trim();
        if (slaveRaw !== "0" && slaveRaw !== "1") return null;

        return {
            hallSensors: values.slice(0, 18),
            temperature: values[18],
            slaveConnected: slaveRaw === "1",
            raw: payload,
        };
    };

    const parseStatusPayload = (
        payload: string,
    ): ParsedStatusPayload | null => {
        const trimmed = payload.trim();

        if (trimmed === "OFF") {
            return { mode: "OFF", raw: payload };
        }

        if (trimmed === "SLEEP") {
            return { mode: "SLEEP", raw: payload };
        }

        const onMatch = /^ON:([0-9A-Fa-f]{4})$/.exec(trimmed);
        if (!onMatch) return null;

        const maskHex = onMatch[1].toUpperCase();
        const mask = Number.parseInt(maskHex, 16);
        const masterMask = mask & 0x007f;
        const slaveMask = (mask >> 7) & 0x007f;

        return {
            mode: "ON",
            maskHex,
            masterMask,
            slaveMask,
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
    $: parsedEntries = topicEntries.map(([topic, payload]) => {
        const isReportTopic = topic.endsWith("/rpt");
        const isStatusTopic = topic.endsWith("/sta");

        return {
            topic,
            payload,
            report: isReportTopic ? parseReportPayload(payload) : null,
            status: isStatusTopic ? parseStatusPayload(payload) : null,
            isReportTopic,
            isStatusTopic,
        };
    });

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

                {#if entry.isReportTopic && entry.report}
                    <table>
                        <thead>
                            <tr>
                                <th>Field</th>
                                <th>Value</th>
                            </tr>
                        </thead>
                        <tbody>
                            {#each entry.report.hallSensors as value, idx}
                                <tr>
                                    <td>Hall {idx + 1}</td>
                                    <td>{value}</td>
                                </tr>
                            {/each}

                            <tr>
                                <td>Temperature (raw)</td>
                                <td>{entry.report.temperature}</td>
                            </tr>

                            <tr>
                                <td>Slave Connected</td>
                                <td
                                    >{entry.report.slaveConnected
                                        ? "true"
                                        : "false"}</td
                                >
                            </tr>
                        </tbody>
                    </table>
                {:else if entry.isStatusTopic && entry.status}
                    <table>
                        <thead>
                            <tr>
                                <th>Field</th>
                                <th>Value</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr>
                                <td>Status</td>
                                <td>{entry.status.mode}</td>
                            </tr>
                            {#if entry.status.mode === "ON"}
                                <tr>
                                    <td>Mask (Hex)</td>
                                    <td>{entry.status.maskHex}</td>
                                </tr>
                                <tr>
                                    <td>Master Bits (0-6)</td>
                                    <td>{entry.status.masterMask}</td>
                                </tr>
                                <tr>
                                    <td>Slave Bits (7-13)</td>
                                    <td>{entry.status.slaveMask}</td>
                                </tr>
                            {/if}
                        </tbody>
                    </table>
                {:else if entry.isReportTopic || entry.isStatusTopic}
                    <p>Invalid payload format: {entry.payload}</p>
                {:else}
                    <p>Raw payload: {entry.payload}</p>
                {/if}
            </div>
        {/each}
    {/if}
</div>

<style>
    .receiver-box {
        width: 100%;
        padding: 8px;
        font-size: 0.82rem;
        overflow: auto;
        color: var(--text-primary);
    }

    .topic-section {
        margin-top: 10px;
        border: 1px solid color-mix(in srgb, var(--separator) 70%, transparent);
        border-radius: 10px;
        padding: 8px;
        background: color-mix(
            in srgb,
            var(--card-bg) 85%,
            var(--bg-elevated) 15%
        );
    }

    h4 {
        margin: 0 0 8px;
        color: var(--text-primary);
    }

    table {
        width: 100%;
        border-collapse: collapse;
    }

    th,
    td {
        border: 1px solid color-mix(in srgb, var(--separator) 72%, transparent);
        padding: 4px 6px;
        text-align: left;
        word-break: break-word;
    }

    th {
        background: color-mix(
            in srgb,
            var(--card-bg) 75%,
            var(--accent-cyan) 25%
        );
    }
</style>
