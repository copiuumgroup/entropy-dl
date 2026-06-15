import axios from 'axios';
import type { Job, SearchResult, Settings, ClearJobsResponse } from '../types';

const baseUrl = import.meta.env.VITE_BACKEND_URL || '';

const api = axios.create({
  baseURL: `${baseUrl}/api`,
  headers: { 'Content-Type': 'application/json' },
});

// ─── Environment ───

export const fetchEnv = (): Promise<import('../types').EnvData> =>
  api.get('/env').then(r => r.data);

export const completeOnboarding = (): Promise<void> =>
  api.post('/onboarding').then(r => r.data);

// ─── Settings ───

export const fetchSettings = (): Promise<Settings> =>
  api.get('/settings').then(r => r.data);

export const saveOutputDir = ({ output_dir }: { output_dir: string }): Promise<Settings> =>
  api.post('/settings', { output_dir }).then(r => r.data);

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

export default api;
