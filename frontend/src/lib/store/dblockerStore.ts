import { writable, get } from 'svelte/store';
import { API_BASE } from '../utils/api';

export interface DBlockerConfig {
    signal_ctrl: boolean;
    signal_gps: boolean;
}

export interface DBlocker {
    id: number;
    name: string;
    serial_numb: string;
    latitude: number;
    longitude: number;
    desc: string;
    angle_start: number;
    config: DBlockerConfig[];
}

// --- STORE ---
export const dblockerStore = writable<DBlocker[]>([]);

// --- CONFIG ---
let pollingInterval: ReturnType<typeof setInterval> | undefined;


// --- READ DATA (GET) ---
export async function fetchDBlockers() {
    try {
        const res = await fetch(`${API_BASE}/api/dblockers`);
        if (!res.ok) throw new Error("Fetch dblockers failed");

        const json = await res.json();
        // Handle wrapped response { data: [...] } or direct array [...]
        const data: DBlocker[] = Array.isArray(json) ? json : (json.data || []);

        // Sort by ID to keep the list order stable
        data.sort((a, b) => a.id - b.id);

        if (JSON.stringify(get(dblockerStore)) !== JSON.stringify(data)) {
            dblockerStore.set(data);
        }
    } catch (err) {
        console.error("Polling Error:", err);
    }
}

export function startPolling(intervalMs = 3000) {
    fetchDBlockers();
    stopPolling();
    pollingInterval = setInterval(fetchDBlockers, intervalMs);
}

export function stopPolling() {
    if (pollingInterval) clearInterval(pollingInterval);
}


// CHANGE DATA (PUT CONFIG)
// This function updates the full config for a dblocker using the real API
export async function updateDBlockerConfig(blockerId: number, config: DBlockerConfig[]) {
    try {
        const payload = {
            id: blockerId,
            config: config
        };

        const res = await fetch(`${API_BASE}/api/dblockers/config`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        if (!res.ok) throw new Error('Failed to update dblocker config');

        // Optionally update the store with the new config if server returns updated data
        // const updatedBlocker = await res.json();
        // dblockerStore.update(items => items.map(b => b.id === blockerId ? { ...b, config: updatedBlocker.config } : b));
    } catch (err) {
        console.error('Failed to update dblocker config:', err);
        alert('Failed to update dblocker config. Check connection.');
    }
}
export async function switchSignal(
    blockerId: number,
    sectorIdx: number,
    type: 'signal_ctrl' | 'signal_gps',
    newValue: boolean
) {
    // A. Optimistic Update: Update UI *immediately* so it feels fast
    dblockerStore.update(items => items.map(b => {
        if (b.id !== blockerId) return b;

        // Create deep copy of config to trigger Svelte update
        const newConfig = [...b.config];
        newConfig[sectorIdx] = { ...newConfig[sectorIdx], [type]: newValue };

        return { ...b, config: newConfig };
    }));

    // B. Send Request to Server
    try {
        const payload = {
            id: blockerId,
            sector: sectorIdx,
            type: type,   // "signal_ctrl" or "signal_gps"
            value: newValue // true or false
        };

        const res = await fetch(`${API_BASE}/api/dblockers/switch`, {
            method: 'POST', // or 'PUT'
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        if (!res.ok) throw new Error("Update failed");

        // Optional: If server returns the new full object, update store again here
        // const updatedBlocker = await res.json();
        // dblockerStore.update(...)

    } catch (err) {
        console.error("Failed to switch signal:", err);

        // C. Rollback: If server failed, flip the switch back!
        dblockerStore.update(items => items.map(b => {
            if (b.id !== blockerId) return b;
            const newConfig = [...b.config];
            newConfig[sectorIdx] = { ...newConfig[sectorIdx], [type]: !newValue }; // Revert
            return { ...b, config: newConfig };
        }));

        alert("Failed to update signal. Check connection.");
    }
}