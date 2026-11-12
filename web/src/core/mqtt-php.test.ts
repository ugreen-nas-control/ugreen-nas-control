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

// What the card shows as the reason the broker cannot be reached: the last line of the publisher's log, while the log is fresh.
test.skipIf(!php)('the connect error is the last log line, only while the log is fresh', async () => {
    const retry = 'mqtt: dial tcp 192.168.40.99:1883: connect: no route to host; retrying in 32s';
    const cases: [log: string, age: number, want: string][] = [
        [`${retry}\n`, 5, 'dial tcp 192.168.40.99:1883: connect: no route to host'],
        [`mqtt: dial tcp x: refused; retrying in 1s\n${retry}\n`, 60, 'dial tcp 192.168.40.99:1883: connect: no route to host'],
        [retry, 120, 'dial tcp 192.168.40.99:1883: connect: no route to host'],
        [`${retry}\n`, 121, ''],
        ['', 0, ''],
        ['mqtt: subscription refused by the broker (SUBACK 0x80); remote commands will not arrive\n', 0, ''],
        [`${retry}\ncommand "reboot" rejected: ts too old\n`, 0, ''],
        ['mqtt: broker refused the connection: bad user name or password; retrying in 4s\n', 0, 'broker refused the connection: bad user name or password'],
        [`mqtt: ${'x'.repeat(301)}; retrying in 1s\n`, 0, '']
    ];
    const script = `require ${JSON.stringify(join(root, 'web/plugin/include/mqtt.php'))};
$cases = json_decode(file_get_contents('php://stdin'), true);
echo json_encode(array_map(fn($c) => mqtt_connect_error($c[0], $c[1]), $cases));`;
    const run = Bun.spawn([php!, '-r', script], { stdin: new Blob([JSON.stringify(cases)]), stdout: 'pipe' });
    const got: string[] = JSON.parse(await new Response(run.stdout).text());
    expect(got).toEqual(cases.map(c => c[2]));
});
