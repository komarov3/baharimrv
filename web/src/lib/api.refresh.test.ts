import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiClient, ApiError } from './api';
import type { AuthSession } from '../types/api';

function session(access: string, refresh: string): AuthSession {
  return {
    sessionToken: access,
    expiresAt: '2030-01-01T00:00:00Z',
    refreshToken: refresh,
    refreshExpiresAt: '2030-01-08T00:00:00Z',
    user: { id: 'u1', email: 'm@example.test', name: 'Manager', role: 'project_manager', status: 'active' } as AuthSession['user'],
  };
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('ApiClient silent refresh', () => {
  let client: ApiClient;
  let fetchMock: ReturnType<typeof vi.fn>;
  let onRenewed: ReturnType<typeof vi.fn>;
  let onExpired: ReturnType<typeof vi.fn>;
  let refreshCalls: number;

  beforeEach(() => {
    client = new ApiClient('');
    client.setToken('old-access');
    client.setRefreshToken('old-refresh');
    refreshCalls = 0;
    onRenewed = vi.fn();
    onExpired = vi.fn();
    client.setSessionHandlers({ onRenewed, onExpired });
    fetchMock = vi.fn(async (url: string | URL | Request, init?: RequestInit) => {
      const path = String(url);
      const auth = new Headers(init?.headers).get('Authorization');
      if (path.endsWith('/auth/refresh')) {
        refreshCalls += 1;
        return jsonResponse(session('new-access', 'new-refresh'));
      }
      if (auth === 'Bearer new-access') return jsonResponse({ ok: true });
      return jsonResponse({ error: { code: 'invalid_session', message: 'expired' } }, 401);
    });
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renews once and retries the original request with the new bearer', async () => {
    const result = await client.request<{ ok: boolean }>('/projects');
    expect(result.ok).toBe(true);
    expect(refreshCalls).toBe(1);
    expect(onRenewed).toHaveBeenCalledTimes(1);
    expect(onRenewed.mock.calls[0][0].sessionToken).toBe('new-access');
    expect(onExpired).not.toHaveBeenCalled();

    await client.request('/projects');
    const lastAuth = new Headers(fetchMock.mock.calls.at(-1)?.[1]?.headers).get('Authorization');
    expect(lastAuth).toBe('Bearer new-access');
    expect(refreshCalls).toBe(1);
  });

  it('sends the stored grant in the refresh body', async () => {
    await client.request('/projects');
    const refreshCall = fetchMock.mock.calls.find(([url]) => String(url).endsWith('/auth/refresh'));
    expect(JSON.parse(String(refreshCall?.[1]?.body))).toEqual({ refreshToken: 'old-refresh' });
  });

  it('expires the session when the grant is rejected', async () => {
    fetchMock.mockImplementation(async (url: string | URL | Request) => {
      if (String(url).endsWith('/auth/refresh')) {
        return jsonResponse({ error: { code: 'invalid_refresh', message: 'dead' } }, 401);
      }
      return jsonResponse({ error: { code: 'invalid_session', message: 'expired' } }, 401);
    });
    await expect(client.request('/projects')).rejects.toMatchObject({ status: 401 });
    expect(onExpired).toHaveBeenCalledTimes(1);
    expect(onRenewed).not.toHaveBeenCalled();

    await expect(client.request('/projects')).rejects.toBeInstanceOf(ApiError);
    const lastAuth = new Headers(fetchMock.mock.calls.at(-1)?.[1]?.headers).get('Authorization');
    expect(lastAuth).toBeNull();
  });

  it('shares one refresh across concurrent 401s', async () => {
    fetchMock.mockImplementation(async (url: string | URL | Request, init?: RequestInit) => {
      const path = String(url);
      const auth = new Headers(init?.headers).get('Authorization');
      if (path.endsWith('/auth/refresh')) {
        refreshCalls += 1;
        await new Promise((resolve) => setTimeout(resolve, 20));
        return jsonResponse(session('new-access', 'new-refresh'));
      }
      if (auth === 'Bearer new-access') return jsonResponse({ ok: true });
      return jsonResponse({ error: { code: 'invalid_session', message: 'expired' } }, 401);
    });
    const results = await Promise.all([
      client.request<{ ok: boolean }>('/projects'),
      client.request<{ ok: boolean }>('/methodologies'),
      client.request<{ ok: boolean }>('/marketplace/listings'),
    ]);
    expect(results.every((r) => r.ok)).toBe(true);
    expect(refreshCalls).toBe(1);
    expect(onRenewed).toHaveBeenCalledTimes(1);
  });

  it('never refreshes on a failed sign-in', async () => {
    await expect(client.request('/auth/login', { method: 'POST', body: '{}' })).rejects.toMatchObject({ status: 401 });
    expect(refreshCalls).toBe(0);
    expect(onExpired).not.toHaveBeenCalled();
  });

  it('keeps the session when the refresh call cannot reach the server', async () => {
    fetchMock.mockImplementation(async (url: string | URL | Request) => {
      if (String(url).endsWith('/auth/refresh')) throw new TypeError('network down');
      return jsonResponse({ error: { code: 'invalid_session', message: 'expired' } }, 401);
    });
    await expect(client.request('/projects')).rejects.toMatchObject({ status: 401 });
    expect(onExpired).not.toHaveBeenCalled();
    expect(onRenewed).not.toHaveBeenCalled();
  });

  it('does not attempt a refresh without a stored grant', async () => {
    client.setRefreshToken(undefined);
    await expect(client.request('/projects')).rejects.toMatchObject({ status: 401 });
    expect(refreshCalls).toBe(0);
    expect(onExpired).not.toHaveBeenCalled();
  });
});