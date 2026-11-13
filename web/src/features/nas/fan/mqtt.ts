// The MQTT card on the Settings page (mqtt_get, mqtt_set): the mqtt_* keys of fan.cfg that the publisher reads (docs/mqtt.md).
// One save bar for the whole card. The password is never loaded: the field stays empty, an empty field keeps the saved one.
import { errorMessage } from '../../../core/api';
import type { MqttResponse, MqttSet, MqttTest, MqttTestResponse } from '../../../core/contract-mqtt';
import { escapeHtml } from '../../../core/html';
import { fieldOfKey, mqttBadKey, mqttTestBadKey } from '../../../core/mqtt';
import { registerFanModule, type FanCtx } from './module';
import type { Key } from '../text';

export interface MqttDraft {
    enabled: boolean;
    host: string;
    port: string;
    tls: boolean;
    user: string;
    pass: string;
    passClear: boolean;
    name: string;
    interval: string;
    control: boolean;
    controlUpdates: boolean;
    controlContainers: boolean;
}

export type MqttField = 'host' | 'port' | 'user' | 'pass' | 'name' | 'interval';

export const draftOf = (saved: MqttResponse): MqttDraft => ({
    enabled: saved.enabled,
    host: saved.host,
    port: saved.port === null ? '' : String(saved.port),
    tls: saved.tls,
    user: saved.user,
    pass: '',
    passClear: false,
    name: saved.name,
    interval: saved.interval === null ? '' : String(saved.interval),
    control: saved.control,
    controlUpdates: saved.control_updates,
    controlContainers: saved.control_containers
});

const flag = (on: boolean): '0' | '1' => (on ? '1' : '0');

export const paramsOf = (draft: MqttDraft): MqttSet => ({
    enabled: flag(draft.enabled),
    tls: flag(draft.tls),
    control: flag(draft.control),
    control_updates: flag(draft.controlUpdates),
    control_containers: flag(draft.controlContainers),
    host: draft.host.trim(),
    port: draft.port.trim(),
    user: draft.user,
    pass: draft.passClear ? '' : draft.pass,
    pass_clear: flag(draft.passClear),
    name: draft.name.trim(),
    interval: draft.interval.trim()
});

/** What the connection test sends: the form as it is, saved or not. */
export const testParamsOf = (draft: MqttDraft): MqttTest => ({
    host: draft.host.trim(),
    port: draft.port.trim(),
    tls: flag(draft.tls),
    user: draft.user,
    pass: draft.passClear ? '' : draft.pass,
    pass_clear: flag(draft.passClear)
});

const FIELDS: readonly MqttField[] = ['host', 'port', 'user', 'pass', 'name', 'interval'];
const CHECKS = { enabled: 'enabled', tls: 'tls', control: 'control', control_updates: 'controlUpdates', control_containers: 'controlContainers' } as const;
const RELOAD_MS = 10_000;

// The switches that let a display act on the NAS ask first; the text names the switch.
const CONFIRM: Record<string, Key> = {
    'unc-mqtt-control': 'mqtt.control',
    'unc-mqtt-control-updates': 'mqtt.control_updates',
    'unc-mqtt-control-containers': 'mqtt.control_containers'
};

const state = {
    saved: null as MqttResponse | null,
    draft: null as MqttDraft | null,
    error: '',
    loading: false,
    loadedAt: 0,
    shown: '',
    testing: false,
    test: null as MqttTestResponse | null
};

const idOf = (name: string): string => `unc-mqtt-${name.replace(/_/g, '-')}`;

function row(ctx: FanCtx, field: MqttField, draft: MqttDraft, saved: MqttResponse): string {
    const { t } = ctx;
    const id = idOf(field);
    const secret = field === 'pass';
    const placeholder = field === 'port' ? String(draft.tls ? 8883 : 1883) : field === 'interval' ? '5' : field === 'name' ? t('mqtt.name_placeholder') : '';
    const attrs = `id="${id}" data-field="${field}" type="${secret ? 'password' : 'text'}" value="${secret ? '' : escapeHtml(draft[field])}" placeholder="${escapeHtml(secret && saved.pass_set ? `${t('mqtt.pass_set')} ••••••••` : placeholder)}" spellcheck="false" autocomplete="${secret ? 'new-password' : 'off'}"${field === 'port' || field === 'interval' ? ' inputmode="numeric"' : ''}`;
    const clear = secret && saved.pass_set ? ctx.ui.chipCheck(`data-pass-clear`, draft.passClear, escapeHtml(t('mqtt.pass_clear'))) : '';
    return `<div class="unc-form">
        <label for="${id}">${escapeHtml(t(`mqtt.${field}`))}</label>
        <div class="unc-mqtt-control"><input ${attrs}${secret && draft.passClear ? ' disabled' : ''}>${clear}${field === 'user' ? '' : `<small class="unc-muted">${escapeHtml(t(`mqtt.${field}_hint`))}</small>`}</div>
    </div>`;
}

function formHtml(ctx: FanCtx, draft: MqttDraft, saved: MqttResponse): string {
    const { ui, t } = ctx;
    const sw = (key: keyof typeof CHECKS, title: Key, hint: Key): string => ui.switchRow({ id: idOf(key), title: t(title), hint: t(hint) });
    const rows = (['host', 'port'] as const).map(field => row(ctx, field, draft, saved)).join('');
    const login = (['user', 'pass'] as const).map(field => row(ctx, field, draft, saved)).join('');
    const naming = (['name', 'interval'] as const).map(field => row(ctx, field, draft, saved)).join('');
    return `${sw('enabled', 'mqtt.enabled', 'mqtt.enabled_hint')}
        <div data-state></div>
        ${rows}
        ${sw('tls', 'mqtt.tls', 'mqtt.tls_hint')}
        ${login}
        <div class="unc-form">
            <span class="unc-label"></span>
            <div class="unc-mqtt-control"><div class="unc-fan-tools-actions" data-test-actions></div><div data-test-result></div><small class="unc-muted">${escapeHtml(t('mqtt.test_hint'))}</small></div>
        </div>
        ${naming}
        ${ui.sectionHead(t('mqtt.control_title'))}
        <p class="unc-muted">${escapeHtml(t('mqtt.control_intro'))}</p>
        ${sw('control', 'mqtt.control', 'mqtt.control_hint')}
        ${sw('control_updates', 'mqtt.control_updates', 'mqtt.control_updates_hint')}
        ${sw('control_containers', 'mqtt.control_containers', 'mqtt.control_containers_hint')}
        <div data-control-warn>${ui.note(escapeHtml(t('mqtt.control_warn')), { tone: 'warn', box: true, tag: 'div', ico: 'alert' })}</div>`;
}

// What the publisher does now: its own region, so a status refresh never touches what the user is typing.
function drawState(ctx: FanCtx): void {
    const box = ctx.el.querySelector('[data-state]');
    const saved = state.saved;
    if (!box || !saved) return;
    const { ui, t } = ctx;
    const failed = saved.running && saved.connection === 'failed';
    const pill = !saved.running
        ? ui.pill({ tone: 'plain', ico: '', text: t('mqtt.stopped') })
        : saved.connection === 'connected'
          ? ui.pill({ tone: 'ok', ico: 'check', text: t('mqtt.connected') })
          : saved.connection === 'connecting'
            ? ui.pill({ tone: 'plain', ico: '', text: t('mqtt.connecting') })
            : failed
              ? ui.pill({ tone: 'warn', ico: 'alert', text: t('mqtt.not_connected') })
              : ui.pill({ tone: 'ok', ico: 'check', text: t('mqtt.running') });
    const problem = !saved.enabled ? '' : !saved.running ? t('mqtt.not_running') : failed ? t('mqtt.unreachable', { error: saved.reason }) : '';
    const warn = problem ? ui.note(escapeHtml(problem), { tone: 'warn', box: true, tag: 'div', ico: 'alert' }) : '';
    box.innerHTML = `<div class="unc-fan-tools-status">${pill}</div>${warn}`;
}

// The connection test: its button and what it found. Its own regions, so a result stays while the form is redrawn.
function drawTest(ctx: FanCtx): void {
    const actions = ctx.el.querySelector('[data-test-actions]');
    const result = ctx.el.querySelector('[data-test-result]');
    if (!actions || !result) return;
    const { ui, t } = ctx;
    actions.innerHTML = ui.btn({
        label: t(state.testing ? 'mqtt.testing' : 'mqtt.test'),
        ico: 'activity',
        attrs: `data-test${state.testing ? ' disabled' : ''}`
    });
    const answer = state.test;
    result.innerHTML = !answer
        ? ''
        : answer.connected
          ? ui.note(escapeHtml(t('mqtt.test_ok')), { tone: 'ok', box: true, tag: 'div', ico: 'check' })
          : ui.note(escapeHtml(t('mqtt.test_failed', { reason: answer.reason ?? '' })), { tone: 'warn', box: true, tag: 'div', ico: 'alert' });
}

async function runTest(ctx: FanCtx): Promise<void> {
    const draft = state.draft;
    if (!draft || state.testing) return;
    const params = testParamsOf(draft);
    const key = mqttTestBadKey(params);
    if (key) {
        const field = fieldOfKey(key);
        ctx.toast(ctx.t('mqtt.invalid', { field: ctx.tRaw(`mqtt.${field ?? 'tls'}`) }), 'error');
        if (field) ctx.el.querySelector<HTMLInputElement>(`#${idOf(field)}`)?.focus();
        return;
    }
    state.testing = true;
    state.test = null;
    drawTest(ctx);
    try {
        state.test = await ctx.api.post('mqtt_test', params);
    } catch (error) {
        ctx.toast(errorMessage(error), 'error');
    }
    state.testing = false;
    drawTest(ctx);
}

function syncControls(ctx: FanCtx): void {
    const draft = state.draft;
    if (!draft) return;
    for (const [name, key] of Object.entries(CHECKS)) {
        const box = ctx.el.querySelector<HTMLInputElement>(`#${idOf(name)}`);
        if (box) box.checked = draft[key];
    }
    const any = draft.control || draft.controlUpdates || draft.controlContainers;
    ctx.el.querySelector('[data-control-warn]')?.toggleAttribute('hidden', !any);
}

function paint(ctx: FanCtx, force = false): void {
    const body = ctx.el.querySelector('[data-body]');
    const { saved, draft } = state;
    if (!body) return;
    if (!saved || !draft) {
        const html = state.error
            ? ctx.ui.note(escapeHtml(ctx.t('tools.load_failed', { error: state.error })), { tone: 'warn', box: true, tag: 'div', ico: 'alert' })
            : ctx.ui.emptyText(ctx.t('panel.reading'));
        if (html !== state.shown) body.innerHTML = html;
        state.shown = html;
        return;
    }
    const html = formHtml(ctx, draft, saved);
    // A refresh never rewrites the controls while the user edits them.
    if (force || html !== state.shown) {
        if (ctx.isDirty() && !force) return;
        body.innerHTML = html;
        state.shown = html;
    }
    syncControls(ctx);
    drawState(ctx);
    drawTest(ctx);
}

// The state of the publisher changes without the user touching the settings.
const sameSettings = (a: MqttResponse, b: MqttResponse): boolean =>
    JSON.stringify({ ...a, running: 0, connection: 0, reason: 0, start_failed: 0 }) ===
    JSON.stringify({ ...b, running: 0, connection: 0, reason: 0, start_failed: 0 });

async function load(ctx: FanCtx): Promise<void> {
    if (state.loading) return;
    state.loading = true;
    try {
        const answer = await ctx.api.get('mqtt_get');
        const before = state.saved;
        state.saved = answer;
        state.error = '';
        state.loadedAt = Date.now();
        if (!state.draft || (!ctx.isDirty() && before && !sameSettings(before, answer))) {
            state.draft = draftOf(answer);
            paint(ctx, true);
        } else {
            paint(ctx);
        }
    } catch (error) {
        state.error = errorMessage(error);
        paint(ctx);
    }
    state.loading = false;
}

function edit(ctx: FanCtx, target: HTMLInputElement): void {
    const draft = state.draft;
    if (!draft) return;
    const field = FIELDS.find(f => target.dataset.field === f);
    const check = (Object.keys(CHECKS) as (keyof typeof CHECKS)[]).find(name => target.id === idOf(name));
    if (field || check === 'tls' || target.hasAttribute('data-pass-clear')) {
        if (field !== 'name' && field !== 'interval') {
            state.test = null;
            ctx.el.querySelector('[data-test-result]')?.replaceChildren();
        }
    }
    if (field) {
        draft[field] = target.value;
        if (field === 'pass' && target.value !== '') draft.passClear = false;
    } else if (check) {
        draft[CHECKS[check]] = target.checked;
        syncControls(ctx);
        // The port hint follows the TLS switch.
        if (check === 'tls') ctx.el.querySelector<HTMLInputElement>(`#${idOf('port')}`)?.setAttribute('placeholder', draft.tls ? '8883' : '1883');
    } else if (target.hasAttribute('data-pass-clear')) {
        draft.passClear = target.checked;
        const pass = ctx.el.querySelector<HTMLInputElement>(`#${idOf('pass')}`);
        if (pass) {
            pass.disabled = target.checked;
            if (target.checked) {
                pass.value = '';
                draft.pass = '';
            }
        }
    } else return;
    ctx.markDirty('mqtt');
}

async function save(ctx: FanCtx): Promise<void> {
    const draft = state.draft;
    if (!draft) return;
    const params = paramsOf(draft);
    const field = fieldOfKey(mqttBadKey(params) ?? '');
    if (field) {
        ctx.toast(ctx.t('mqtt.invalid', { field: ctx.tRaw(`mqtt.${field}`) }), 'error');
        ctx.el.querySelector<HTMLInputElement>(`#${idOf(field)}`)?.focus();
        return;
    }
    let reply: MqttResponse | null = null;
    await ctx.save(async () => {
        reply = await ctx.api.post('mqtt_set', params);
    }, ctx.t('save.done'));
    const answer = reply as MqttResponse | null;
    if (!answer) return;
    state.saved = answer;
    state.loadedAt = Date.now();
    if (answer.start_failed) ctx.toast(ctx.t('mqtt.start_failed', { error: answer.start_failed }), 'error');
    if (!ctx.isDirty()) {
        state.draft = draftOf(answer);
        paint(ctx, true);
    } else {
        drawState(ctx);
    }
}

registerFanModule({
    id: 'mqtt',
    host: 'settings',
    order: 60,
    render: ctx => `<article class="unc-card unc-mqtt">
        ${ctx.ui.cardHead(ctx.t('mqtt.title'), { ico: 'net' })}
        <p class="unc-muted">${escapeHtml(ctx.t('mqtt.hint'))}</p>
        <div class="unc-fan-tools-body" data-body></div>
        ${ctx.ui.saveDock('mqtt', 'unc-mqtt-save')}
    </article>`,
    bind: ctx => {
        const onEdit = (event: Event): void => {
            if (event.target instanceof HTMLInputElement) edit(ctx, event.target);
        };
        // A checkbox fires input and then change; only change handles it, after the confirmation.
        ctx.el.addEventListener('input', event => {
            if (event.target instanceof HTMLInputElement && event.target.type !== 'checkbox') edit(ctx, event.target);
        });
        ctx.el.addEventListener('change', event => {
            const target = event.target;
            if (!(target instanceof HTMLInputElement) || target.type !== 'checkbox') return;
            const what = CONFIRM[target.id];
            if (what && target.checked && !window.confirm(ctx.t('mqtt.control_confirm', { what: ctx.t(what) }))) {
                target.checked = false;
                return;
            }
            onEdit(event);
        });
        ctx.el.addEventListener('click', event => {
            const target = event.target instanceof Element ? event.target : null;
            if (target?.closest('[data-discard="mqtt"]')) {
                if (state.saved) state.draft = draftOf(state.saved);
                ctx.clearDirty();
                paint(ctx, true);
            } else if (target?.closest('[data-test]')) {
                void runTest(ctx);
            } else if (target?.closest('#unc-mqtt-save')) {
                void save(ctx);
            }
        });
    },
    // Only the open Settings page asks; the running state is read again every few seconds.
    update: (_status, ctx) => {
        if (!ctx.el.closest('.unc-view')?.classList.contains('active')) return;
        if (!state.saved || (!ctx.isDirty() && Date.now() - state.loadedAt > RELOAD_MS)) void load(ctx);
    }
});
