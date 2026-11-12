<?php

declare(strict_types=1);

// MQTT publisher settings (api.php actions mqtt_get, mqtt_set, mqtt_test): the mqtt_* keys of fan.cfg that
// backend/internal/mqtt/config.go reads (docs/mqtt.md). They work in a v1 and a v2 file, so a save never touches
// cfg_version. The password never leaves the NAS: mqtt_get only says whether one is set. A save applies at once: the
// publisher stops and, if switched on, starts again. Uses respond(), run(), read_settings(), save_settings(), CTL,
// FAN_TOOL and CONFIG_FILE from api.php.

// The publisher's own state (backend/internal/mqtt/status.go): {"state":"connecting|connected|failed","reason":"...","time":N}.
// It exists while the publisher runs.
const MQTT_STATUS = '/var/run/ugreen-nas-control-mqtt.status';

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

// The text fields of a save: field => [fan.cfg key, check]. An empty value is always allowed (back to the default).
function mqtt_rules(): array
{
    return [
        'host' => ['mqtt_host', fn(string $v): bool => preg_match('/^[A-Za-z0-9._-]{1,253}$/D', $v) === 1],
        'port' => ['mqtt_port', fn(string $v): bool => preg_match('/^[1-9][0-9]{0,4}$/D', $v) === 1 && (int) $v <= 65535],
        'user' => ['mqtt_user', fn(string $v): bool => mqtt_login_ok($v)],
        'name' => ['mqtt_name', fn(string $v): bool => preg_match('/^[A-Za-z0-9_-]{1,64}$/D', $v) === 1],
        'interval' => ['mqtt_interval', fn(string $v): bool => preg_match('/^[0-9]{1,2}$/D', $v) === 1 && (int) $v >= 2 && (int) $v <= 60],
    ];
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

    foreach (mqtt_rules() as $field => [$key, $valid]) {
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

// Text of the publisher or of a tool for the page: printable ASCII only (a cut inside a multibyte character would make json_encode
// fail and the whole answer empty), at most 300 characters.
function mqtt_text(string $text): string
{
    return substr((string) preg_replace('/[^\x20-\x7e]/', '', $text), 0, 300);
}

function mqtt_running(): bool
{
    return run(CTL, ['mqtt-status'])['code'] === 0;
}

// What the publisher is doing: off (not running), connecting, connected, failed (with the reason it keeps retrying for) or
// unknown (it runs but keeps no state, an older one). $status: the content of MQTT_STATUS, '' when there is none.
function mqtt_connection(bool $running, string $status): array
{
    if (!$running) {
        return ['connection' => 'off', 'reason' => ''];
    }

    $data = json_decode($status, true);
    $state = is_array($data) ? ($data['state'] ?? null) : null;

    if (!in_array($state, ['connecting', 'connected', 'failed'], true)) {
        return ['connection' => 'unknown', 'reason' => ''];
    }

    $reason = $state === 'failed' && is_string($data['reason'] ?? null) ? $data['reason'] : '';

    return ['connection' => $state, 'reason' => mqtt_text($reason)];
}

// running, connection and reason as mqtt_get answers them.
function mqtt_status(): array
{
    $running = mqtt_running();
    $content = $running && is_file(MQTT_STATUS) ? (string) @file_get_contents(MQTT_STATUS, false, null, 0, 4096) : '';

    return ['running' => $running] + mqtt_connection($running, $content);
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
        $reply['start_failed'] = mqtt_text($start['message']);
    }

    respond($reply);
}

// The fan tool's `mqtt-test`: the settings as JSON on stdin, one answer on stdout.
function mqtt_test_run(array $request): array
{
    $process = @proc_open(
        [FAN_TOOL, 'mqtt-test', '--stdin'],
        [0 => ['pipe', 'r'], 1 => ['pipe', 'w'], 2 => ['pipe', 'w']],
        $pipes,
        '/',
        [],
    );

    if (!is_resource($process)) {
        respond(['ok' => false, 'error' => 'The fan tool cannot be started'], 500);
    }

    @fwrite($pipes[0], (string) json_encode($request));
    fclose($pipes[0]);
    $output = (string) stream_get_contents($pipes[1]);
    fclose($pipes[1]);
    $errors = (string) stream_get_contents($pipes[2]);
    fclose($pipes[2]);
    $code = proc_close($process);
    $data = json_decode($output, true);

    if ($code !== 0 || !is_array($data)) {
        respond(['ok' => false, 'error' => trim((string) preg_replace('/^ERROR:\s*/m', '', $errors)) ?: 'The fan tool gave no answer'], 409);
    }

    return $data;
}

// POST mqtt_test host, port, tls, user, pass, pass_clear: connects once with these values (they need not be saved) and
// says whether the broker took it. An empty pass uses the saved one while host and user are the saved ones, so the page
// never needs the password; the saved password does not go to another broker.
function mqtt_test(array $input): never
{
    $request = ['host' => '', 'port' => 0, 'tls' => false, 'user' => '', 'pass' => ''];

    foreach (mqtt_rules() as $field => [$key, $valid]) {
        if (!in_array($field, ['host', 'port', 'user'], true)) {
            continue;
        }

        $value = $input[$field] ?? '';

        if (!is_string($value) || ($value !== '' && !$valid($value))) {
            respond(['ok' => false, 'error' => 'Invalid value for ' . $key], 400);
        }

        $request[$field] = $field === 'port' ? (int) $value : $value;
    }

    $tls = $input['tls'] ?? null;
    $pass = $input['pass'] ?? '';

    if (($tls !== '0' && $tls !== '1') || !is_string($pass) || ($pass !== '' && !mqtt_login_ok($pass))) {
        respond(['ok' => false, 'error' => 'Invalid value for ' . ($tls !== '0' && $tls !== '1' ? 'mqtt_tls' : 'mqtt_pass')], 400);
    }

    if ($request['host'] === '') {
        respond(['ok' => false, 'error' => 'Invalid value for mqtt_host'], 400);
    }

    $saved = read_settings();

    if ($pass === '' && ($input['pass_clear'] ?? '0') !== '1' && ($saved['mqtt_host'] ?? '') === $request['host'] && ($saved['mqtt_user'] ?? '') === $request['user']) {
        $pass = $saved['mqtt_pass'] ?? '';
    }

    $request['tls'] = $tls === '1';
    $request['pass'] = $pass;
    $answer = mqtt_test_run($request);

    $reply = ['ok' => true, 'connected' => ($answer['connected'] ?? false) === true];

    if (!$reply['connected']) {
        $reply['reason'] = mqtt_text((string) ($answer['reason'] ?? ''));
    }

    respond($reply);
}
