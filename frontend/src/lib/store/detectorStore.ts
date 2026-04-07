import { writable } from 'svelte/store';
import { API_BASE } from '../utils/api';
import { authFetch } from './authStore';

export interface DroneDetector {
    id: number;
    name: string;
    latitude: number;
    longitude: number;
    host: string;
    port: number;
    status: string;
    last_seen: string;
}

export const detectorStore = writable<DroneDetector[]>([]);

let detectorPollTimer: ReturnType<typeof setInterval> | undefined;

export async function fetchDetectors() {
    try {
        const res = await authFetch(`${API_BASE}/api/detectors`);
        if (!res.ok) throw new Error("Fetch detectors failed");
        const json = await res.json();
        const data: DroneDetector[] = Array.isArray(json) ? json : (json.data || []);
        detectorStore.set(data);
    } catch (err) {
        console.error("Detector fetch error:", err);
    }
}

export function startDetectorPolling(intervalMs = 10000) {
    fetchDetectors();
    stopDetectorPolling();
    detectorPollTimer = setInterval(fetchDetectors, intervalMs);
}

export function stopDetectorPolling() {
    if (detectorPollTimer) {
        clearInterval(detectorPollTimer);
        detectorPollTimer = undefined;
    }
}
