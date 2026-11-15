// The save check of the MQTT settings, the same the PHP applies (web/plugin/include/mqtt.php, mqtt_collect; mqtt-php.test.ts
// runs both on one table). The page checks before it sends, the demo answers with it.
import type { MqttSet, MqttTest } from './contract-mqtt';

// Login strings are written between quotes into fan.cfg: no control characters, quote, backslash or ${.
const loginOk = (value: string): boolean => new TextEncoder().encode(value).length <= 256 && !/[\x00-\x1f\x7f"\\]|\$\{/.test(value);

/** An IPv6 address without brackets or zone, as PHP's FILTER_VALIDATE_IP takes it (mqtt-php.test.ts runs both on one table). */
export function isIPv6(value: string): boolean {
    if (value.length > 45 || !/^[0-9A-Fa-f:.]+$/.test(value) || (value.match(/:/g) ?? []).length < 2) return false;
    let rest = value;
    const v4 = /(\d{1,3}(?:\.\d{1,3}){3})$/.exec(value);
    if (v4) {
        const octets = v4[1]!.split('.');
        if (!octets.every(o => /^(0|[1-9]\d{0,2})$/.test(o) && Number(o) <= 255) || value[value.length - v4[1]!.length - 1] !== ':') return false;
        rest = `${value.slice(0, value.length - v4[1]!.length)}0:0`;
    }
    const halves = rest.split('::');
    if (halves.length > 2) return false;
    const groups = (text: string): string[] => (text === '' ? [] : text.split(':'));
    const [head, tail] = [groups(halves[0]!), halves.length === 2 ? groups(halves[1]!) : []];
    if (![...head, ...tail].every(g => /^[0-9A-Fa-f]{1,4}$/.test(g))) return false;
    return halves.length === 1 ? head.length === 8 : head.length + tail.length <= 7;
}

const hostOk = (value: string): boolean => /^[A-Za-z0-9._-]{1,253}$/.test(value) || isIPv6(value);

const FLAGS = {
    enabled: 'mqtt_enabled',
    tls: 'mqtt_tls',
    control: 'mqtt_control',
    control_updates: 'mqtt_control_updates',
    control_containers: 'mqtt_control_containers'
} as const;

const TEXTS: [field: 'host' | 'port' | 'user' | 'name' | 'interval', key: string, valid: (value: string) => boolean][] = [
    ['host', 'mqtt_host', hostOk],
    ['port', 'mqtt_port', v => /^[1-9][0-9]{0,4}$/.test(v) && Number(v) <= 65535],
    ['user', 'mqtt_user', loginOk],
    ['name', 'mqtt_name', v => /^[A-Za-z0-9_-]{1,64}$/.test(v)],
    ['interval', 'mqtt_interval', v => /^[0-9]{1,2}$/.test(v) && Number(v) >= 2 && Number(v) <= 60]
];

/** The fan.cfg key the PHP would refuse in a save, null when it takes all of it. */
export function mqttBadKey(input: Partial<Record<keyof MqttSet, string>>): string | null {
    for (const [field, key] of Object.entries(FLAGS)) {
        const value = input[field as keyof typeof FLAGS];
        if (value !== '0' && value !== '1') return key;
    }
    for (const [field, key, valid] of TEXTS) {
        const value = input[field] ?? '';
        if (value !== '' && !valid(value)) return key;
    }
    if (input.enabled === '1' && (input.host ?? '') === '') return 'mqtt_host';
    const pass = input.pass ?? '';
    return pass !== '' && !loginOk(pass) ? 'mqtt_pass' : null;
}

const FIELD_OF_KEY: Record<string, 'host' | 'port' | 'user' | 'pass' | 'name' | 'interval'> = {
    mqtt_host: 'host',
    mqtt_port: 'port',
    mqtt_user: 'user',
    mqtt_pass: 'pass',
    mqtt_name: 'name',
    mqtt_interval: 'interval'
};

/** The input field behind a refused key; null for a switch (the page cannot send a bad one). */
export const fieldOfKey = (key: string): 'host' | 'port' | 'user' | 'pass' | 'name' | 'interval' | null => FIELD_OF_KEY[key] ?? null;

/** The fan.cfg key the PHP refuses in a connection test (host, port, user, tls, pass), null when it takes all of it. */
export function mqttTestBadKey(input: Partial<Record<keyof MqttTest, string>>): string | null {
    for (const [field, key, valid] of TEXTS) {
        if (field === 'name' || field === 'interval') continue;
        const value = input[field] ?? '';
        if (value !== '' && !valid(value)) return key;
    }
    if (input.tls !== '0' && input.tls !== '1') return 'mqtt_tls';
    if ((input.host ?? '') === '') return 'mqtt_host';
    const pass = input.pass ?? '';
    return pass !== '' && !loginOk(pass) ? 'mqtt_pass' : null;
}
