import { writable, get } from 'svelte/store';
import { API_BASE } from '../utils/api';
import { authFetch } from './authStore';

export interface DBlockerConfig {
    signal_ctrl: boolean;
    signal_gps: boolean;
}

export interface SectorCurrents {
    ctrl1: number;
    ctrl2: number;
    gps: number;
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
    preset_config: DBlockerConfig[] | null;
}

// --- STORE ---
export const dblockerStore = writable<DBlocker[]>([]);
export const expandedDblockerId = writable<number | null>(null);

// --- CONFIG ---
let pollingInterval: ReturnType<typeof setInterval> | undefined;


// --- READ DATA (GET) ---
export async function fetchDBlockers() {
    try {
        const res = await authFetch(`${API_BASE}/api/dblockers`);
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

        const res = await authFetch(`${API_BASE}/api/dblockers/config`, {
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
export async function turnOffAll(blockerId: number) {
    try {
        const res = await authFetch(`${API_BASE}/api/dblockers/config/off/${blockerId}`);
        if (!res.ok) throw new Error('Failed to turn off all');
        await fetchDBlockers();
    } catch (err) {
        console.error('Failed to turn off all:', err);
        alert('Failed to turn off all. Check connection.');
    }
}

export async function presetOn(blockerId: number) {
    try {
        const res = await authFetch(`${API_BASE}/api/dblockers/config/preset/${blockerId}`);
        if (!res.ok) throw new Error('Failed to apply preset ON');
        await fetchDBlockers();
    } catch (err) {
        console.error('Failed to apply preset ON:', err);
        alert('Failed to apply preset ON. Check connection.');
    }
}

