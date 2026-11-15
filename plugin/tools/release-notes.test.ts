import { expect, test } from 'bun:test';
import { contributors, releaseNotes } from './release-notes';

const commit = (name: string, email: string, body = '') => `${name}\0${email}\0${body}\x1e\n`;
const changelog = '# Changelog\n\n## 5.7.0\n\n- MQTT settings.\n- Fix confirmed by [tester](https://github.com/tester).\n\n## 5.6.0\n\n- Old change.\n';

test('credits Alex within a PR opened by Jens and preserves reporter acknowledgements', () => {
    const log = commit('Jens', '77627854+j3n57h0m45@users.noreply.github.com') + commit('Alexander', '23248850+alexanderschubert@users.noreply.github.com');
    const notes = releaseNotes('5.7.0', changelog, log, 'ugreen-nas-control/ugreen-nas-control');
    expect(notes).toContain('[Alexander](https://github.com/alexanderschubert)');
    expect(notes).toContain('[Jens](https://github.com/j3n57h0m45)');
    expect(notes).toContain('[tester](https://github.com/tester)');
    expect(notes).not.toContain('Old change');
});

test('includes co-authors retained in squash commits, deduplicates GitHub identities and excludes bots', () => {
    const log =
        commit('Jens', 'j3n57h0m45@users.noreply.github.com', 'Change\n\nCo-authored-by: Alex <alexanderschubert@users.noreply.github.com>\n') +
        commit('Alexander', '23248850+alexanderschubert@users.noreply.github.com') +
        commit('github-actions[bot]', '41898282+github-actions[bot]@users.noreply.github.com');
    expect(contributors(log)).toEqual(['[Alex](https://github.com/alexanderschubert)', '[Jens](https://github.com/j3n57h0m45)']);
});

test('keeps authors without a GitHub noreply address without exposing their email', () => {
    expect(contributors(commit('A [name]', 'private@example.com'))).toEqual(['A \\[name\\]']);
});

test('fails instead of generating incomplete or empty notes', () => {
    expect(() => contributors('bad record')).toThrow('Malformed');
    expect(() => releaseNotes('5.8.0', changelog, '', 'owner/repo')).toThrow('No changelog');
    expect(() => releaseNotes('5.7.0', changelog, '', 'owner/repo')).toThrow('No release contributors');
});
