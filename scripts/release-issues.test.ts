import { expect, test } from 'bun:test';
import { closeReleasedIssues, readyToClose } from './release-issues';

const issue = { number: 183, state: 'open', labels: [{ name: 'waiting for release' }] };
const release = { draft: false, prerelease: false, published_at: '2026-11-14T12:00:00Z' as string | null, html_url: 'https://example.com/releases/v5.7.0' };

test('only fully resolved open issues qualify', () => {
    expect(readyToClose(issue)).toBe(true);
    expect(readyToClose({ ...issue, state: 'closed' })).toBe(false);
    expect(readyToClose({ ...issue, labels: [] })).toBe(false);
    expect(readyToClose({ ...issue, pull_request: {} })).toBe(false);
    for (const name of ['waiting for reply', 'partially fixed in next', 'on hold', 'needs hardware test']) {
        expect(readyToClose({ ...issue, labels: [...issue.labels, { name }] })).toBe(false);
    }
});

function api(options: { release?: typeof release; commented?: boolean; blockAfterComment?: boolean; failComment?: boolean; labelledAt?: string } = {}) {
    const writes: { method: string; path: string; body: unknown }[] = [];
    let commented = options.commented ?? false;
    return {
        writes,
        request: async (method: string, path: string, body?: Record<string, unknown>): Promise<unknown> => {
            if (method !== 'GET') {
                if (method === 'POST' && options.failComment) throw new Error('comment failed');
                writes.push({ method, path, body });
                if (method === 'POST') commented = true;
                return null;
            }
            if (path.startsWith('releases/')) return options.release ?? release;
            if (path.startsWith('issues?')) return [issue];
            if (path.includes('/events?')) {
                return [{ event: 'labeled', label: { name: 'waiting for release' }, created_at: options.labelledAt ?? '2026-11-13T12:00:00Z' }];
            }
            if (path.includes('/comments?')) return commented ? [{ body: '<!-- stable-release:v5.7.0 -->' }] : [];
            if (path === 'issues/183') {
                return options.blockAfterComment && commented ? { ...issue, labels: [...issue.labels, { name: 'waiting for reply' }] } : issue;
            }
            throw new Error(`unexpected ${method} ${path}`);
        }
    };
}

test('publication is required before any issue writes', async () => {
    for (const overrides of [{ draft: true }, { prerelease: true }, { published_at: null }]) {
        const mock = api({ release: { ...release, ...overrides } });
        await expect(closeReleasedIssues('v5.7.0', mock.request)).rejects.toThrow('not a published stable release');
        expect(mock.writes).toEqual([]);
    }
    const mock = api();
    await expect(closeReleasedIssues('v5.7.0-next.123', mock.request)).rejects.toThrow('stable release tag');
    expect(mock.writes).toEqual([]);
});

test('comments with the release link, closes as completed, then removes the waiting label', async () => {
    const mock = api();
    await closeReleasedIssues('v5.7.0', mock.request);
    expect(mock.writes.map(write => write.method)).toEqual(['POST', 'PATCH', 'DELETE']);
    expect(mock.writes[0]?.body).toEqual({ body: expect.stringContaining(release.html_url) });
    expect(mock.writes[1]?.body).toEqual({ state: 'closed', state_reason: 'completed' });
});

test('retry after a successful comment does not post a duplicate', async () => {
    const mock = api({ commented: true });
    await closeReleasedIssues('v5.7.0', mock.request);
    expect(mock.writes.map(write => write.method)).toEqual(['PATCH', 'DELETE']);
});

test('new follow-up labels prevent closure even after the release comment', async () => {
    const mock = api({ blockAfterComment: true });
    await closeReleasedIssues('v5.7.0', mock.request);
    expect(mock.writes.map(write => write.method)).toEqual(['POST']);
});

test('a failed release comment leaves the issue open and labelled for retry', async () => {
    const mock = api({ failComment: true });
    await expect(closeReleasedIssues('v5.7.0', mock.request)).rejects.toThrow('comment failed');
    expect(mock.writes).toEqual([]);
});

test('retrying an old release does not close a fix labelled after publication', async () => {
    const mock = api({ labelledAt: '2026-11-15T12:00:00Z' });
    await closeReleasedIssues('v5.7.0', mock.request);
    expect(mock.writes).toEqual([]);
});

test('paginates issues, label events and comments before deciding whether to close', async () => {
    const mock = api();
    const seen: string[] = [];
    await closeReleasedIssues('v5.7.0', async (method, path, body) => {
        seen.push(path);
        if (method === 'GET') {
            if (path.startsWith('issues?')) return path.endsWith('page=1') ? Array.from({ length: 100 }, () => ({ ...issue, labels: [] })) : [issue];
            if (path.includes('/events?')) {
                return path.endsWith('page=1')
                    ? Array.from({ length: 100 }, () => ({ event: 'assigned' }))
                    : [{ event: 'labeled', label: { name: 'waiting for release' }, created_at: '2026-11-13T12:00:00Z' }];
            }
            if (path.includes('/comments?')) {
                return path.endsWith('page=1') ? Array.from({ length: 100 }, () => ({ body: 'Older comment' })) : [{ body: '<!-- stable-release:v5.7.0 -->' }];
            }
        }
        return mock.request(method, path, body);
    });
    expect(seen).toContain('issues?state=open&labels=waiting%20for%20release&per_page=100&page=2');
    expect(seen).toContain('issues/183/events?per_page=100&page=2');
    expect(seen).toContain('issues/183/comments?per_page=100&page=2');
    expect(mock.writes.map(write => write.method)).toEqual(['PATCH', 'DELETE']);
});
