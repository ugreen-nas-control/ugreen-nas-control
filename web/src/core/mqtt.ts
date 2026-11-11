// The save check of the MQTT settings, the same the PHP applies (web/plugin/include/mqtt.php, mqtt_collect; mqtt-php.test.ts
// runs both on one table). The page checks before it sends, the demo answers with it.
import type { MqttSet } from './contract-mqtt';

// Login strings are written between quotes into fan.cfg: no control characters, quote, backslash or ${.
const loginOk = (value: string): boolean => new TextEncoder().encode(value).length <= 256 && !/[\x00-\x1f\x7f"\\]|\$\{/.test(value);

const FLAGS = {
    enabled: 'mqtt_enabled',
    tls: 'mqtt_tls',
    control: 'mqtt_control',
    control_updates: 'mqtt_control_updates',
    control_containers: 'mqtt_control_containers'
} as const;

const TEXTS: [field: 'host' | 'port' | 'user' | 'name' | 'interval', key: string, valid: (value: string) => boolean][] = [
    ['host', 'mqtt_host', v => /^[A-Za-z0-9._-]{1,253}$/.test(v)],
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
