import { $ } from 'bun';

interface Issue {
    number: number;
    state: string;
    state_reason?: string;
    labels: { name: string }[];
    pull_request?: unknown;
}

type Request = (method: string, path: string, body?: Record<string, unknown>) => Promise<unknown>;
const blocked = ['waiting for reply', 'partially fixed in next', 'on hold', 'needs hardware test'];

export function readyToClose(issue: Issue): boolean {
    return issue.state === 'open' && readyForReleaseNote(issue);
}

function readyForReleaseNote(issue: Issue): boolean {
    const labels = issue.labels.map(label => label.name);
    const completed = issue.state === 'open' || (issue.state === 'closed' && issue.state_reason === 'completed');
    return completed && !issue.pull_request && labels.includes('waiting for release') && !blocked.some(label => labels.includes(label));
}

async function list<T>(request: Request, path: string): Promise<T[]> {
    const all: T[] = [];
    for (let page = 1; ; page++) {
        const rows = (await request('GET', `${path}${path.includes('?') ? '&' : '?'}per_page=100&page=${page}`)) as T[];
        all.push(...rows);
        if (rows.length < 100) return all;
    }
}

export async function closeReleasedIssues(tag: string, request: Request): Promise<void> {
    if (!/^v[1-9]\.[0-9]\.[0-9]$/.test(tag)) throw new Error('expected a stable release tag vX.Y.Z');
    const release = (await request('GET', `releases/tags/${tag}`)) as {
        draft: boolean;
        prerelease: boolean;
        published_at: string | null;
        html_url: string;
    };
    if (release.draft || release.prerelease || !release.published_at) throw new Error(`${tag} is not a published stable release`);
    const issues = await list<Issue>(request, 'issues?state=all&labels=waiting%20for%20release');
    for (const issue of issues.filter(readyForReleaseNote)) {
        const path = `issues/${issue.number}`;
        if (!readyForReleaseNote((await request('GET', path)) as Issue)) continue;
        const events = await list<{ event: string; label?: { name: string }; created_at: string }>(request, `${path}/events`);
        const labelled = events.filter(event => event.event === 'labeled' && event.label?.name === 'waiting for release').at(-1);
        // Retrying an older release must not close fixes queued for a later release.
        if (!labelled || Date.parse(labelled.created_at) > Date.parse(release.published_at)) continue;
        const marker = `<!-- stable-release:${tag} -->`;
        const comments = await list<{ body: string }>(request, `${path}/comments`);
        if (!comments.some(comment => comment.body.includes(marker))) {
            await request('POST', `${path}/comments`, {
                body: `${marker}\nThe fix is now available in [${tag}](${release.html_url}). Update under Plugins. This issue is completed; please reply if the problem remains.`
            });
        }
        // A reporter or maintainer may have changed the labels while the comment was being posted.
        const current = (await request('GET', path)) as Issue;
        if (!readyForReleaseNote(current)) continue;
        if (readyToClose(current)) await request('PATCH', path, { state: 'closed', state_reason: 'completed' });
        await request('DELETE', `${path}/labels/waiting%20for%20release`);
        console.log(`Completed release cleanup for #${issue.number} after ${tag}`);
    }
}

if (import.meta.main) {
    const repo = process.env.GITHUB_REPOSITORY;
    if (!repo) throw new Error('GITHUB_REPOSITORY is not set');
    await closeReleasedIssues(process.argv[2] ?? '', async (method, path, body) => {
        const args = ['api', '--method', method, `repos/${repo}/${path}`];
        if (body) args.push('--input', '-');
        const result = body ? await $`printf '%s' ${JSON.stringify(body)} | gh ${args}`.quiet() : await $`gh ${args}`.quiet();
        const text = result.stdout.toString();
        return text ? JSON.parse(text) : null;
    });
}
