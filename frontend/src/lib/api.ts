import axios from 'axios';
import type { Job, SearchResult, Settings, ClearJobsResponse, LibraryRoots, LibraryEntry, LibraryRoot, AuthUser, SetupResponse, User } from '../types';

const baseUrl = import.meta.env.VITE_BACKEND_URL || '';

const api = axios.create({
  baseURL: `${baseUrl}/api`,
  headers: { 'Content-Type': 'application/json' },
});

// ─── 401 Interceptor ───
// Registers a callback that fires when any API call returns 401.
// The LoginScreen sets this to force a re-check (which drops back to login).

type AuthLostCallback = () => void;
let _onAuthLost: AuthLostCallback | null = null;

export function onAuthLost(cb: AuthLostCallback): () => void {
  _onAuthLost = cb;
  return () => { _onAuthLost = null; };
}

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401 && _onAuthLost) {
      _onAuthLost();
    }
    return Promise.reject(error);
  },
);

// ─── Environment ───

export const fetchEnv = (): Promise<import('../types').EnvData> =>
  api.get('/env').then(r => r.data);

// ─── Onboarding ───

export const completeOnboarding = (): Promise<void> =>
  api.post('/onboarding').then(r => r.data);

// ─── Auth ───

// fetchMe checks who the current user is. Returns the AuthUser on success,
// or throws on 401. In loopback mode the backend returns loopback:true.
export const fetchMe = (): Promise<AuthUser> =>
  api.get('/me').then(r => r.data);

// setup creates the first admin account. Only works when no users exist
// (returns 409 Conflict if setup already completed).
export const setup = (username: string, password: string): Promise<SetupResponse> =>
  api.post('/setup', { username, password }).then(r => r.data);

// login authenticates with username/password. Backend sets the session cookie.
// In loopback mode returns a synthetic admin without requiring credentials.
export const login = (username: string, password: string): Promise<AuthUser> =>
  api.post('/login', { username, password }).then(r => r.data);

// logout invalidates the current session and clears the cookie.
export const logout = (): Promise<void> =>
  api.post('/logout').then(r => r.data);

// ─── User management (admin only) ───

// fetchUsers lists every named account. Admin-only; returns 400 in loopback mode.
export const fetchUsers = (): Promise<User[]> =>
  api.get('/users').then(r => r.data.users || []);

// createUser adds a new account. is_admin defaults to false.
export const createUser = ({ username, password, is_admin }: {
  username: string;
  password: string;
  is_admin: boolean;
}): Promise<User> =>
  api.post('/users', { username, password, is_admin }).then(r => r.data.user);

// deleteUser removes an account. The backend refuses to delete the last admin.
export const deleteUser = (username: string): Promise<void> =>
  api.delete(`/users/${encodeURIComponent(username)}`).then(r => r.data);

// ─── Settings ───

export const fetchSettings = (): Promise<Settings> =>
  api.get('/settings').then(r => r.data);

export const saveOutputDirs = (dirs: { audio_dir: string; video_dir: string }): Promise<Settings> =>
  api.post('/settings', dirs).then(r => r.data);

export const setSmartRouting = (enabled: boolean): Promise<{ smart_routing: boolean }> =>
  api.post('/smart-routing', { enabled }).then(r => r.data);

// ─── Search ───

export const searchItems = (source: string, query: string, limit: number = 15): Promise<SearchResult[]> =>
  api.post('/search', { source, query, limit }).then(r => r.data.results || []);

export const cleanUrl = (text: string): Promise<string[]> =>
  api.post('/clean-url', { text }).then(r => r.data.urls || []);

// ─── Jobs ───

export const fetchJobs = (): Promise<Job[]> =>
  api.get('/jobs').then(r => r.data.jobs || []);

export const createJobs = ({ urls, items, options }: {
  urls?: string[];
  items?: Partial<SearchResult>[];
  options: import('../types').JobOptions;
}): Promise<Job[]> =>
  api.post('/jobs', { urls, items, options }).then(r => r.data.jobs || []);

export const retryJob = (id: string): Promise<Job> =>
  api.post(`/jobs/${id}/retry`).then(r => r.data);

export const deleteJob = (id: string): Promise<void> =>
  api.delete(`/jobs/${id}`).then(r => r.data);

export const clearJobs = (what: string): Promise<ClearJobsResponse> =>
  api.post('/jobs/clear', { what }).then(r => r.data);

export const openFolder = (id: string): Promise<void> =>
  api.post(`/jobs/${id}/open-folder`).then(r => r.data);

// ─── Concurrency ───

export const fetchConcurrency = (): Promise<number> =>
  api.get('/concurrency').then(r => r.data.workers);

export const updateConcurrency = (workers: number): Promise<number> =>
  api.post('/concurrency', { workers }).then(r => r.data.workers);

// ─── Tools ───

export const updateTools = (): Promise<void> =>
  api.post('/tools/update').then(r => r.data);

// ─── Shutdown ───

export const shutdown = (): Promise<void> =>
  api.post('/shutdown').then(r => r.data);

// ─── Library ───

export const fetchLibrary = (): Promise<LibraryRoots> =>
  api.get('/library').then(r => r.data);

export const fetchLibraryDir = (root: LibraryRoot, path: string = ''): Promise<LibraryEntry[]> =>
  api.get('/library/dir', { params: { root, path } }).then(r => r.data.entries || []);

// libraryFileURL builds a direct streaming URL for a media file. Bypasses the
// axios instance because <audio>/<video> elements need a plain URL, and the
// browser will send the session cookie automatically (same-origin).
export const libraryFileURL = (root: LibraryRoot, path: string): string =>
  `${baseUrl}/api/library/file?root=${encodeURIComponent(root)}&path=${encodeURIComponent(path)}`;

export default api;
