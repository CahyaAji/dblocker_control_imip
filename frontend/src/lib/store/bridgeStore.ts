import { writable } from 'svelte/store';
import { API_BASE } from '../utils/api';

// Holds the latest payload for every MQTT topic received via SSE
export const bridgeStore = writable<Record<string, string>>({});

let source: EventSource | null = null;
let refCount = 0;

export function subscribeBridge() {
    refCount++;
    if (source) return;

    source = new EventSource(`${API_BASE}/events`);

    source.onmessage = (ev: MessageEvent<string>) => {
        try {
            const data = JSON.parse(ev.data) as { topic?: string; payload?: string };
            const topic = data.topic?.trim();
            if (!topic) return;
            const payload = data.payload ?? '';
            bridgeStore.update(current => ({ ...current, [topic]: payload }));
        } catch {
            // ignore malformed events
        }
    };
}

export function unsubscribeBridge() {
    refCount--;
    if (refCount <= 0) {
        source?.close();
        source = null;
        refCount = 0;
    }
}
