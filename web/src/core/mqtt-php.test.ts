import { expect, test } from 'bun:test';
import { join } from 'node:path';
import type { MqttSet } from './contract-mqtt';
import { mqttBadKey } from './mqtt';

const root = join(import.meta.dir, '../../..');
const php = Bun.which('php');

const base: MqttSet = {
    enabled: '1',
    tls: '0',
    control: '0',
    control_updates: '0',
    control_containers: '0',
    host: 'broker.lan',
    port: '1883',
    user: 'nas',
    pass: '',
    pass_clear: '0',
    name: 'tower',
    interval: '10'
};

const table: [what: string, change: Partial<MqttSet>][] = [
    ['all valid', {}],
    ['everything empty while off', { enabled: '0', host: '', port: '', user: '', name: '', interval: '' }],
    ['on without host', { host: '' }],
    ['flag 2', { tls: '2' as '1' }],
    ['flag empty', { control: '' as '1' }],
    ['host with space', { host: 'a b' }],
    ['host with port', { host: 'a:1883' }],
    ['host ipv6 ::1', { host: '::1' }],
    ['host ipv6 ::', { host: '::' }],
    ['host ipv6 fe80::1', { host: 'fe80::1' }],
    ['host ipv6 2001:db8::10', { host: '2001:db8::10' }],
    ['host ipv6 2001:0db8:0000:0000:0000:0000:', { host: '2001:0db8:0000:0000:0000:0000:0000:0010' }],
    ['host ipv6 2001:DB8::A', { host: '2001:DB8::A' }],
    ['host ipv6 ::ffff:192.168.1.5', { host: '::ffff:192.168.1.5' }],
    ['host ipv6 1:2:3:4:5:6:1.2.3.4', { host: '1:2:3:4:5:6:1.2.3.4' }],
    ['host ipv6 1:2:3:4:5:6:7:8', { host: '1:2:3:4:5:6:7:8' }],
    ['host ipv6 1::8', { host: '1::8' }],
    ['host ipv6 a:a:a:a:a:a:a:a', { host: 'a:a:a:a:a:a:a:a' }],
    ['host not ipv6 [::1]', { host: '[::1]' }],
    ['host not ipv6 :::', { host: ':::' }],
    ['host not ipv6 1:2:3:4:5:6:7:8:9', { host: '1:2:3:4:5:6:7:8:9' }],
    ['host not ipv6 1::2::3', { host: '1::2::3' }],
    ['host not ipv6 fe80::1%eth0', { host: 'fe80::1%eth0' }],
    ['host not ipv6 g::1', { host: 'g::1' }],
    ['host not ipv6 :1:2:3:4:5:6:7', { host: ':1:2:3:4:5:6:7' }],
    ['host not ipv6 1:2:3:4:5:6:7', { host: '1:2:3:4:5:6:7' }],
    ['host not ipv6 ::1.2.3', { host: '::1.2.3' }],
    ['host not ipv6 ::1.2.3.256', { host: '::1.2.3.256' }],
    ['host not ipv6 ::1.02.3.4', { host: '::1.02.3.4' }],
    ['host not ipv6 12345::1', { host: '12345::1' }],
    ['host not ipv6 ::1 ', { host: '::1 ' }],
    ['host not ipv6 1:2:3:4:5:6:7::8', { host: '1:2:3:4:5:6:7::8' }],
    ['host not ipv6 a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:', { host: 'a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a' }],
    ['host not ipv6 192.168.1.5:1883', { host: '192.168.1.5:1883' }],
    ['host not ipv6 broker:lan', { host: 'broker:lan' }],
    ['host 253 characters', { host: 'a'.repeat(253) }],
    ['host 254 characters', { host: 'a'.repeat(254) }],
    ['port 1', { port: '1' }],
    ['port 65535', { port: '65535' }],
    ['port 65536', { port: '65536' }],
    ['port 0', { port: '0' }],
    ['port 01883', { port: '01883' }],
    ['port +1883', { port: '+1883' }],
    ['port text', { port: '18x' }],
    ['interval 2', { interval: '2' }],
    ['interval 60', { interval: '60' }],
    ['interval 1', { interval: '1' }],
    ['interval 61', { interval: '61' }],
    ['interval 005', { interval: '005' }],
    ['name 64 characters', { name: 'a'.repeat(64) }],
    ['name 65 characters', { name: 'a'.repeat(65) }],
    ['name with slash', { name: 'a/b' }],
    ['name with plus', { name: 'a+b' }],
    ['user with umlaut', { user: 'Jürgen' }],
    ['user 256 bytes', { user: 'a'.repeat(256) }],
    ['user 257 bytes', { user: 'a'.repeat(257) }],
    ['user 129 umlauts (258 bytes)', { user: 'ü'.repeat(129) }],
    ['user with quote', { user: 'a"b' }],
    ['user with backslash', { user: 'a\\b' }],
    ['user with newline', { user: 'a\nb' }],
    ['user with tab', { user: 'a\tb' }],
    ['user with DEL', { user: 'a\x7fb' }],
    ['pass with symbols', { pass: "p@ss w0rd!#;'$%&" }],
    ['pass with ${', { pass: 'p${HOME}' }],
    ['pass with lone $', { pass: 'cost$5' }],
    ['pass with quote', { pass: 'p"w' }],
    ['pass with backslash', { pass: 'p\\w' }],
    ['pass 257 bytes', { pass: 'a'.repeat(257) }]
];

test.skipIf(!php)('the page check refuses what the PHP save refuses, and nothing else', async () => {
    const inputs = table.map(([, change]) => ({ ...base, ...change }));
    const script = `require ${JSON.stringify(join(root, 'web/plugin/include/mqtt.php'))};
$cases = json_decode(file_get_contents('php://stdin'), true);
echo json_encode(array_map(function ($input) { $bad = null; return mqtt_collect($input, $bad) === null ? $bad : null; }, $cases));`;
    const run = Bun.spawn([php!, '-r', script], { stdin: new Blob([JSON.stringify(inputs)]), stdout: 'pipe' });
    const got: (string | null)[] = JSON.parse(await new Response(run.stdout).text());
    const want = inputs.map(input => mqttBadKey(input));
    expect(table.map(([what], i) => `${what}: ${got[i]}`)).toEqual(table.map(([what], i) => `${what}: ${want[i]}`));
});

// The settings page shows the state the publisher keeps (backend/internal/mqtt/status.go) only while it runs.
test.skipIf(!php)('the connection is the publisher state while it runs, and nothing it does not know', async () => {
    const cases: [running: boolean, file: string, want: { connection: string; reason: string }][] = [
        [false, '{"state":"connected"}', { connection: 'off', reason: '' }],
        [true, '{"state":"connected","time":1}', { connection: 'connected', reason: '' }],
        [true, '{"state":"connecting"}', { connection: 'connecting', reason: '' }],
        [true, '{"state":"failed","reason":"dial tcp x:1883: i/o timeout"}', { connection: 'failed', reason: 'dial tcp x:1883: i/o timeout' }],
        [true, '{"state":"failed"}', { connection: 'failed', reason: '' }],
        [true, '{"state":"failed","reason":5}', { connection: 'failed', reason: '' }],
        [true, '{"state":"connected","reason":"stale"}', { connection: 'connected', reason: '' }],
        [true, `{"state":"failed","reason":"${'x'.repeat(400)}"}`, { connection: 'failed', reason: 'x'.repeat(300) }],
        [true, '{"state":"weird"}', { connection: 'unknown', reason: '' }],
        [true, '{"state":["failed"]}', { connection: 'unknown', reason: '' }],
        [true, 'not json', { connection: 'unknown', reason: '' }],
        [true, '', { connection: 'unknown', reason: '' }]
    ];
    const script = `require ${JSON.stringify(join(root, 'web/plugin/include/mqtt.php'))};
$cases = json_decode(file_get_contents('php://stdin'), true);
echo json_encode(array_map(fn($c) => mqtt_connection($c[0], $c[1]), $cases));`;
    const run = Bun.spawn([php!, '-r', script], { stdin: new Blob([JSON.stringify(cases)]), stdout: 'pipe' });
    const got: unknown[] = JSON.parse(await new Response(run.stdout).text());
    expect(got).toEqual(cases.map(c => c[2]));
});

// Text the page gets from the publisher or a tool must stay valid in JSON, whatever it holds.
test.skipIf(!php)('texts for the page are printable ASCII, at most 300 characters', async () => {
    const cases: [string, string][] = [
        ['dial tcp 10.0.0.1:1883: i/o timeout', 'dial tcp 10.0.0.1:1883: i/o timeout'],
        ['', ''],
        ['a\u0000b\nc\u007fd', 'abcd'],
        ['Verbindung abgelehnt: größer', 'Verbindung abgelehnt: grer'],
        ['ü'.repeat(200), ''],
        ['x'.repeat(400), 'x'.repeat(300)],
        ['ü' + 'x'.repeat(400), 'x'.repeat(300)]
    ];
    const script = `require ${JSON.stringify(join(root, 'web/plugin/include/mqtt.php'))};
$cases = json_decode(file_get_contents('php://stdin'), true);
echo json_encode(array_map(fn($c) => mqtt_text($c[0]), $cases));`;
    const run = Bun.spawn([php!, '-r', script], { stdin: new Blob([JSON.stringify(cases)]), stdout: 'pipe' });
    const got: string[] = JSON.parse(await new Response(run.stdout).text());
    expect(got).toEqual(cases.map(c => c[1]));
});
