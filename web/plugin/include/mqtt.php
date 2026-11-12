<?php

declare(strict_types=1);

// MQTT publisher settings (api.php actions mqtt_get, mqtt_set): the mqtt_* keys of fan.cfg that backend/internal/mqtt/config.go
// reads (docs/mqtt.md). They work in a v1 and a v2 file, so a save never touches cfg_version. The password never leaves
// the NAS: mqtt_get only says whether one is set. A save applies at once: the publisher stops and, if switched on,
// starts again. Uses respond(), run(), read_settings(), save_settings(), CTL and CONFIG_FILE from api.php.

// The publisher's log, replaced on every start (backend/internal/nasctl, UGREEN_PUB_LOG). A failed connection is retried at
// least every 60 s and logs each try; a log older than this means the connection came back.
const MQTT_LOG = '/var/log/ugreen-nas-control-mqtt.log';
const MQTT_LOG_FRESH = 120;

const MQTT_FLAGS = [
    'enabled' => 'mqtt_enabled',
    'tls' => 'mqtt_tls',
    'control' => 'mqtt_control',
    'control_updates' => 'mqtt_control_updates',
    'control_containers' => 'mqtt_control_containers',
];

// Login strings are written as key="value" and read back by parse_ini_file, which unescapes \ and expands ${...}.
function mqtt_login_ok(string $value): bool
{
    return strlen($value) <= 256 && preg_match('//u', $value) === 1 && preg_match('/[\x00-\x1f\x7f"\\\\]|\$\{/', $value) !== 1;
}

// The values of a save as fan.cfg keys: [changes, remove] (an empty field goes back to its default), or null with $bad
// naming the key that failed.
function mqtt_collect(array $input, ?string &$bad): ?array
{
    $changes = [];
    $remove = [];

    foreach (MQTT_FLAGS as $field => $key) {
        $value = $input[$field] ?? null;

        if ($value !== '0' && $value !== '1') {
            $bad = $key;

            return null;
        }

        $changes[$key] = $value;
    }

    $fields = [
        'host' => ['mqtt_host', fn(string $v): bool => preg_match('/^[A-Za-z0-9._-]{1,253}$/D', $v) === 1],
        'port' => ['mqtt_port', fn(string $v): bool => preg_match('/^[1-9][0-9]{0,4}$/D', $v) === 1 && (int) $v <= 65535],
        'user' => ['mqtt_user', fn(string $v): bool => mqtt_login_ok($v)],
        'name' => ['mqtt_name', fn(string $v): bool => preg_match('/^[A-Za-z0-9_-]{1,64}$/D', $v) === 1],
        'interval' => ['mqtt_interval', fn(string $v): bool => preg_match('/^[0-9]{1,2}$/D', $v) === 1 && (int) $v >= 2 && (int) $v <= 60],
    ];

    foreach ($fields as $field => [$key, $valid]) {
        $value = $input[$field] ?? '';

        if (!is_string($value) || ($value !== '' && !$valid($value))) {
            $bad = $key;

            return null;
        }

        if ($value === '') {
            $remove[] = $key;
        } else {
            $changes[$key] = $value;
        }
    }

    if ($changes['mqtt_enabled'] === '1' && !isset($changes['mqtt_host'])) {
        $bad = 'mqtt_host';

        return null;
    }

    // The password is only sent when it changes; pass_clear=1 deletes it.
    $pass = $input['pass'] ?? '';

    if (!is_string($pass) || ($pass !== '' && !mqtt_login_ok($pass))) {
        $bad = 'mqtt_pass';

        return null;
    }

    if (($input['pass_clear'] ?? '0') === '1') {
        $remove[] = 'mqtt_pass';
    } elseif ($pass !== '') {
        $changes['mqtt_pass'] = $pass;
    }

    return [$changes, $remove];
}

// The settings as the daemon reads them: a value it would not take shows as unset.
function mqtt_view(array $config): array
{
    $port = (int) ($config['mqtt_port'] ?? '');
    $interval = (int) ($config['mqtt_interval'] ?? '');

    return [
        'enabled' => ($config['mqtt_enabled'] ?? '') === '1',
        'host' => $config['mqtt_host'] ?? '',
        'port' => $port > 0 && $port < 65536 ? $port : null,
        'tls' => ($config['mqtt_tls'] ?? '') === '1',
        'user' => $config['mqtt_user'] ?? '',
        'pass_set' => ($config['mqtt_pass'] ?? '') !== '',
        'name' => preg_replace('/[^a-zA-Z0-9_-]/', '', $config['mqtt_name'] ?? ''),
        'interval' => $interval >= 2 && $interval <= 60 ? $interval : null,
        'control' => ($config['mqtt_control'] ?? '') === '1',
        'control_updates' => ($config['mqtt_control_updates'] ?? '') === '1',
        'control_containers' => ($config['mqtt_control_containers'] ?? '') === '1',
    ];
}

function mqtt_running(): bool
{
    return run(CTL, ['mqtt-status'])['code'] === 0;
}

// Why the publisher cannot reach the broker, from the last line of its log ("mqtt: <error>; retrying in 8s"); empty when the
// log is older than MQTT_LOG_FRESH seconds, ends otherwise or has no such line. $age: seconds since the log was written.
function mqtt_connect_error(string $log, int $age): string
{
    if ($age > MQTT_LOG_FRESH) {
        return '';
    }

    $lines = explode("\n", rtrim($log, "\n"));

    return preg_match('/^mqtt: (.{1,300}); retrying in [0-9]+s$/Du', end($lines), $match) === 1 ? $match[1] : '';
}

function mqtt_status(): array
{
    $log = is_file(MQTT_LOG) ? (string) @file_get_contents(MQTT_LOG, false, null, max(0, (int) @filesize(MQTT_LOG) - 4096)) : '';
    $age = $log === '' ? PHP_INT_MAX : max(0, time() - (int) @filemtime(MQTT_LOG));

    return ['running' => mqtt_running(), 'last_error' => mqtt_connect_error($log, $age)];
}

// GET mqtt_get
function mqtt_get(): never
{
    respond(['ok' => true] + mqtt_view(read_settings()) + mqtt_status());
}

// POST mqtt_set enabled, tls, control, control_updates, control_containers = 0|1; host, port, user, name, interval
// (empty: default); pass (empty: keep) or pass_clear=1.
function mqtt_set(array $input): never
{
    $bad = null;
    $collected = mqtt_collect($input, $bad);

    if ($collected === null) {
        respond(['ok' => false, 'error' => 'Invalid value for ' . $bad], 400);
    }

    if (!save_settings($collected[0], $collected[1])) {
        respond(['ok' => false, 'error' => CONFIG_FILE . ' is not writable'], 500);
    }

    // A running publisher keeps the settings it started with, so it always restarts; ctl starts it only when switched on.
    run(CTL, ['mqtt-stop']);
    $start = run(CTL, ['mqtt-start']);
    $reply = ['ok' => true] + mqtt_view(read_settings()) + mqtt_status();

    if ($start['code'] !== 0) {
        $reply['start_failed'] = $start['message'];
    }

    respond($reply);
}
