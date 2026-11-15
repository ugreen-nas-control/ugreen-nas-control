import { $ } from 'bun';
import { parseChangelog } from './changes';

const escape = (text: string) => text.replace(/[\\`*_[\]<>]/g, '\\$&');

export function contributors(log: string): string[] {
    const people = new Map<string, string>();
    function add(name: string, email: string) {
        name = name.trim();
        email = email.trim().toLowerCase();
        if (/\[bot\]/i.test(name + email)) return;
        const login = email.match(/^(?:\d+\+)?([a-z\d-]+)@users\.noreply\.github\.com$/)?.[1];
        const key = login ?? email;
        if (!name || !email) throw new Error('Missing contributor identity');
        if (!people.has(key)) people.set(key, login ? `[${escape(name)}](https://github.com/${login})` : escape(name));
    }
    for (const record of log.split('\x1e').map(s => s.trim()).filter(Boolean)) {
        const fields = record.split('\0');
        if (fields.length !== 3) throw new Error('Malformed contributor log');
        const [name, email, body] = fields;
        add(name, email);
        for (const coauthor of body.matchAll(/^Co-authored-by:\s*(.+?)\s*<([^<>]+)>\s*$/gim)) add(coauthor[1], coauthor[2]);
    }
    return [...people.values()].sort((a, b) => a.localeCompare(b, 'en'));
}

export function releaseNotes(version: string, changelog: string, log: string, repository: string): string {
    const release = parseChangelog(changelog).find(r => r.version === version);
    if (!release?.lines.length) throw new Error(`No changelog entries for ${version}`);
    const people = contributors(log);
    if (!people.length) throw new Error('No release contributors found');
    return `Install: https://raw.githubusercontent.com/${repository}/main/plugin/ugreen-nas-control.plg

## What's changed

${release.lines.map(line => `- ${line}`).join('\n')}

## Commit contributors

${people.map(person => `- ${person}`).join('\n')}

Includes commit authors and credited co-authors across the release, including work incorporated through another contributor's PR. Reporter and testing acknowledgements are preserved in the changes above.
`;
}

if (import.meta.main) {
    const [version, target] = process.argv.slice(2);
    const repository = process.env.GITHUB_REPOSITORY;
    if (!version || !target || !repository) throw new Error('Usage: release-notes.ts <version> <target>; GITHUB_REPOSITORY is required');
    const changelog = await Bun.file('CHANGELOG.md').text();
    const releases = parseChangelog(changelog);
    if (releases[0]?.version !== version || !releases[1]) throw new Error('Expected the folded release and its predecessor in CHANGELOG.md');
    const base = `v${releases[1].version}`;
    const log = await $`git log --no-merges --format=%aN%x00%aE%x00%B%x1e ${`${base}..${target}`} --`.quiet().text();
    process.stdout.write(releaseNotes(version, changelog, log, repository));
}
