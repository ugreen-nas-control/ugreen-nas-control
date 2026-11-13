import { expect, test } from 'bun:test';
import type { MqttResponse } from '../../../core/contract-mqtt';
import { mqttBadKey, mqttTestBadKey } from '../../../core/mqtt';
import { draftOf, paramsOf, testParamsOf } from './mqtt';

const saved: MqttResponse = {
    ok: true,
    enabled: true,
    host: 'broker.lan',
    port: 8883,
    tls: true,
    user: 'nas',
    pass_set: true,
    name: 'tower',
    interval: 10,
    control: true,
    control_updates: false,
    control_containers: true,
    running: true,
    connection: 'connected',
    reason: ''
};

test('the draft starts from the saved settings and sends them back unchanged, without a password', () => {
    expect(paramsOf(draftOf(saved))).toEqual({
        enabled: '1',
        tls: '1',
        control: '1',
        control_updates: '0',
        control_containers: '1',
        host: 'broker.lan',
        port: '8883',
        user: 'nas',
        pass: '',
        pass_clear: '0',
        name: 'tower',
        interval: '10'
    });
    expect(mqttBadKey(paramsOf(draftOf(saved)))).toBeNull();
});

test('unset values stay empty so the defaults apply', () => {
    const draft = draftOf({ ...saved, port: null, interval: null, name: '', user: '', pass_set: false });
    expect(paramsOf(draft)).toMatchObject({ port: '', interval: '', name: '', user: '', pass: '' });
});

test('a new password is sent; deleting the saved one sends no password', () => {
    expect(paramsOf({ ...draftOf(saved), pass: 'new' })).toMatchObject({ pass: 'new', pass_clear: '0' });
    expect(paramsOf({ ...draftOf(saved), pass: 'new', passClear: true })).toMatchObject({ pass: '', pass_clear: '1' });
});

test('spaces around host, port, name and interval are dropped before the check', () => {
    expect(paramsOf({ ...draftOf(saved), host: ' broker.lan ', port: ' 1883 ', name: ' tower ', interval: ' 5 ' })).toMatchObject({
        host: 'broker.lan',
        port: '1883',
        name: 'tower',
        interval: '5'
    });
});

test('the check names the first key the PHP would refuse', () => {
    const base = paramsOf(draftOf(saved));
    expect(mqttBadKey({ ...base, host: '' })).toBe('mqtt_host');
    expect(mqttBadKey({ ...base, enabled: '0', host: '' })).toBeNull();
    expect(mqttBadKey({ ...base, port: '70000' })).toBe('mqtt_port');
    expect(mqttBadKey({ ...base, interval: '1' })).toBe('mqtt_interval');
    expect(mqttBadKey({ ...base, name: 'a b' })).toBe('mqtt_name');
    expect(mqttBadKey({ ...base, pass: 'a"b' })).toBe('mqtt_pass');
    expect(mqttBadKey({ ...base, user: 'a\\b' })).toBe('mqtt_user');
});

test('the connection test sends the form as it is and keeps the saved password out of it', () => {
    const draft = { ...draftOf(saved), host: ' other.lan ', pass: 'typed' };
    expect(testParamsOf(draft)).toEqual({ host: 'other.lan', port: '8883', tls: '1', user: 'nas', pass: 'typed', pass_clear: '0' });
    expect(testParamsOf(draftOf(saved)).pass).toBe('');
    expect(testParamsOf({ ...draft, passClear: true })).toMatchObject({ pass: '', pass_clear: '1' });
});

test('the connection test checks only what it sends', () => {
    const ok = testParamsOf(draftOf(saved));
    expect(mqttTestBadKey(ok)).toBeNull();
    expect(mqttTestBadKey({ ...ok, host: '' })).toBe('mqtt_host');
    expect(mqttTestBadKey({ ...ok, host: 'a b' })).toBe('mqtt_host');
    expect(mqttTestBadKey({ ...ok, port: '0' })).toBe('mqtt_port');
    expect(mqttTestBadKey({ ...ok, port: '' })).toBeNull();
    expect(mqttTestBadKey({ ...ok, user: 'a"b' })).toBe('mqtt_user');
    expect(mqttTestBadKey({ ...ok, pass: 'p${x}' })).toBe('mqtt_pass');
    expect(mqttTestBadKey({ ...ok, tls: '2' as '1' })).toBe('mqtt_tls');
    // name and interval are not part of a test
    expect(mqttTestBadKey({ ...ok, name: 'a b', interval: '1' } as never)).toBeNull();
});
