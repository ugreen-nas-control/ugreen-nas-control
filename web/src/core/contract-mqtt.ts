/*
 * MQTT publisher settings (web/plugin/include/mqtt.php, docs/mqtt.md): the mqtt_* keys of fan.cfg.
 * The password never comes back, only whether one is set. A value the daemon would not take (port, interval, name) reads as null / "".
 */
import type { ApiOk } from './contract';

export type MqttConnection = 'off' | 'connecting' | 'connected' | 'failed' | 'unknown';

/** GET mqtt_get; POST mqtt_set answers with the saved settings. */
export type MqttResponse = ApiOk & {
    enabled: boolean;
    host: string;
    /** null: the default (1883, 8883 with TLS). */
    port: number | null;
    tls: boolean;
    user: string;
    pass_set: boolean;
    /** Empty: the NAS host name. */
    name: string;
    /** Seconds, 2 to 60; null: 5. */
    interval: number | null;
    control: boolean;
    control_updates: boolean;
    control_containers: boolean;
    /** The publisher process is up now. */
    running: boolean;
    /** What the publisher is doing: `unknown` when it runs but keeps no state (an older one). */
    connection: MqttConnection;
    /** `failed` only: why it cannot reach the broker, as its log words it; it keeps retrying. */
    reason: string;
    /** After a set only: `ugreen-nas-ctl mqtt-start` failed; the settings are saved. */
    start_failed?: string;
};

/**
 * POST mqtt_set. Flags are '0' or '1'; an empty host (only while off), port, user, name or interval goes back to the default.
 * `pass` empty keeps the saved password, `pass_clear=1` deletes it. Login strings: UTF-8, up to 256 bytes, none of `"` `\` `${`.
 */
export interface MqttSet {
    enabled: '0' | '1';
    tls: '0' | '1';
    control: '0' | '1';
    control_updates: '0' | '1';
    control_containers: '0' | '1';
    host: string;
    port: string;
    user: string;
    pass: string;
    pass_clear: '0' | '1';
    name: string;
    interval: string;
}

/**
 * POST mqtt_test: connects once with these values (they need not be saved) and hangs up. `pass` empty uses the saved
 * password while host and user are the saved ones; `pass_clear` '1' never does. Takes up to 20 s.
 */
export interface MqttTest {
    host: string;
    port: string;
    tls: '0' | '1';
    user: string;
    pass: string;
    pass_clear: '0' | '1';
}

/** The broker took the connection, or `reason` says why not (the broker's refusal, a network error). */
export type MqttTestResponse = ApiOk & { connected: boolean; reason?: string };
