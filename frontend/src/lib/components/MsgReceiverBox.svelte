<script lang="ts">
    import { onMount } from "svelte";
    import { API_BASE } from "../utils/api";

    type BridgeEvent = {
        topic?: string;
        payload?: string;
    };


    type CurrentData = {
        raw: number;
        amps: number;
    };

    type ParsedPayload = {
        currents: CurrentData[];
        temperatureRaw: number;
        temperatureCelsius: number | null;
        digitalRaw: string;
        digital: boolean | null;
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


    // --- MATH CONVERSIONS ---
    const calculateTemperatureC = (rawADC: number): number | null => {
        if (rawADC <= 0 || rawADC >= 1023) return null;

        const R_FIXED = 51000.0;
        const R0 = 100000.0;
        const B_COEFFICIENT = 3950.0;
        const T0 = 298.15;

        const resistance = R_FIXED * (1023.0 / rawADC - 1.0);
        let steinhart = resistance / R0;
        steinhart = Math.log(steinhart);
        steinhart /= B_COEFFICIENT;
        steinhart += 1.0 / T0;
        steinhart = 1.0 / steinhart;

        return steinhart - 273.15;
    };

    const calculateCurrentA = (rawADC: number): number => {
        const VCC = 3.3;
        const voltage = rawADC * (VCC / 1023.0);
        const vZero = VCC / 2.0;
        const sensitivity = 0.0396;
        return (voltage - vZero) / sensitivity;
    };

    // --- PAYLOAD PARSING ---
    const parseReportPayload = (payload: string): ParsedPayload | null => {
        const [numericPart, digitalPart] = payload.split("|");
        if (!numericPart) return null;

        const values = numericPart
            .split(",")
            .map((item) => Number(item.trim()))
            .filter((item) => !Number.isNaN(item));

        if (values.length < 19) return null;

        const digitalRaw = (digitalPart ?? "").trim();
        const digital =
            digitalRaw === "1" ? true : digitalRaw === "0" ? false : null;

        const temperatureRaw = values[18];

        // Map the first 18 raw values to objects containing both raw and calculated amps
        const currents = values.slice(0, 18).map(raw => ({
            raw,
            amps: calculateCurrentA(raw)
        }));

        return {
            currents,
            temperatureRaw,
            temperatureCelsius: calculateTemperatureC(temperatureRaw),
            digitalRaw,
            digital,
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
        return {
            topic,
            payload,
            parsed: isReportTopic ? parseReportPayload(payload) : null,
            isReportTopic,
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
                {#if entry.isReportTopic && entry.parsed}
                    <table class="current-table">
                        <thead>
                            <tr>
                                <th>Current #</th>
                                <th>Raw Value</th>
                                <th>Converted (A)</th>
                            </tr>
                        </thead>
                        <tbody>
                            {#each entry.parsed.currents as current, idx}
                                <tr>
                                    <td>Current {idx}</td>
                                    <td>{current.raw}</td>
                                    <td>{current.amps.toFixed(3)}</td>
                                </tr>
                            {/each}
                            <tr>
                                <td>Temperature</td>
                                <td>{entry.parsed.temperatureRaw}</td>
                                <td>{entry.parsed.temperatureCelsius !== null ? entry.parsed.temperatureCelsius.toFixed(2) + ' °C' : 'N/A'}</td>
                            </tr>
                        </tbody>
                    </table>
                    <div class="digital-info">
                        <span>Digital Raw: {entry.parsed.digitalRaw}</span>
                        <span>Digital: {entry.parsed.digital === null ? 'N/A' : entry.parsed.digital ? 'true' : 'false'}</span>
                    </div>
                {:else if entry.isReportTopic}
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

    table.current-table {
        width: 100%;
        border-collapse: collapse;
        margin-bottom: 8px;
    }

    .current-table th,
    .current-table td {
        border: 1px solid color-mix(in srgb, var(--separator) 72%, transparent);
        padding: 4px 6px;
        text-align: left;
        word-break: break-word;
    }

    .current-table th {
        background: color-mix(
            in srgb,
            var(--card-bg) 75%,
            var(--accent-cyan) 25%
        );
    }

    .digital-info {
        font-size: 0.9em;
        color: var(--text-secondary, #888);
        margin-bottom: 4px;
        display: flex;
        gap: 1.5em;
    }
</style>
