import { writable, get } from 'svelte/store';
import { API_BASE } from '../utils/api';

export interface AuthUser {
    id: number;
    username: string;
    is_admin: boolean;
}

interface AuthState {
    token: string | null;
    user: AuthUser | null;
    loading: boolean;
}

const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('auth') : null;
const initial: AuthState = stored
    ? { ...JSON.parse(stored), loading: false }
    : { token: null, user: null, loading: false };

export const authStore = writable<AuthState>(initial);

// Persist token + user to localStorage
authStore.subscribe((value) => {
    if (typeof localStorage !== 'undefined') {
        localStorage.setItem('auth', JSON.stringify({ token: value.token, user: value.user }));
    }
});

export function getAuthToken(): string | null {
    return get(authStore).token;
}

export function isLoggedIn(): boolean {
    return get(authStore).token !== null;
}

export function isAdmin(): boolean {
    return get(authStore).user?.is_admin ?? false;
}

export async function login(username: string, password: string): Promise<{ ok: boolean; error?: string }> {
    try {
        const res = await fetch(`${API_BASE}/api/auth/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
        });

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            return { ok: false, error: data.error || 'Login failed' };
        }

        const data = await res.json();
        authStore.set({ token: data.token, user: data.user, loading: false });
        return { ok: true };
    } catch {
        return { ok: false, error: 'Network error' };
    }
}

export function logout() {
    authStore.set({ token: null, user: null, loading: false });
}

// Verify stored token is still valid
export async function verifyToken(): Promise<boolean> {
    const { token } = get(authStore);
    if (!token) return false;

    try {
        const res = await fetch(`${API_BASE}/api/auth/me`, {
            headers: { Authorization: `Bearer ${token}` },
        });

        if (!res.ok) {
            logout();
            return false;
        }

        const data = await res.json();
        authStore.update(s => ({ ...s, user: data.user }));
        return true;
    } catch {
        logout();
        return false;
    }
}

// Helper: fetch with auth header
export async function authFetch(url: string, init: RequestInit = {}): Promise<Response> {
    const { token } = get(authStore);
    const headers = new Headers(init.headers);
    if (token) {
        headers.set('Authorization', `Bearer ${token}`);
    }
    const res = await fetch(url, { ...init, headers });

    if (res.status === 401) {
        logout();
    }

    return res;
}
