import { writable } from 'svelte/store';

// Define the shape of your settings
interface AppSettings {
    mapStyle: 'normal' | 'hybrid';
    sidebarExpanded: boolean;
    sidebarWidth: number;
    theme: 'light' | 'dark';
}

// Default settings (for first-time users)
const DEFAULT_SETTINGS: AppSettings = {
    mapStyle: 'normal',
    sidebarExpanded: false,
    sidebarWidth: 300,
    theme: 'light'
};

// Load from LocalStorage (with SSR safety check)
const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('app-settings') : null;
let initialValue = stored ? JSON.parse(stored) : null;

if (!initialValue) {
    initialValue = { ...DEFAULT_SETTINGS };
    if (typeof window !== 'undefined' && window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
        initialValue.theme = 'dark';
    }
} else if (!initialValue.theme) {
    if (typeof window !== 'undefined' && window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
        initialValue.theme = 'dark';
    } else {
        initialValue.theme = 'light';
    }
}

// Create the store
export const settings = writable<AppSettings>(initialValue);

// Auto-save: Whenever any value changes, write to LocalStorage
settings.subscribe((value) => {
    if (typeof localStorage !== 'undefined') {
        localStorage.setItem('app-settings', JSON.stringify(value));
    }
});