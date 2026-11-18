/** Colour steps of the temperature bars: [warm, hot] in °C. */
export const LIMITS = {
    cpu: [70, 85],
    disk: [45, 50],
    nvme: [60, 70],
    other: [55, 70]
} as const;

export type TempKind = keyof typeof LIMITS;
export type TempLevel = 'hot' | 'warm' | 'ok' | '';

/**
 * The unit temperatures are shown in (the setting `ui_unit`, which every status carries), and whether the other unit is shown
 * beside it (`ui_unit_both`). Only the page converts: the daemon, fan.cfg (curves, limits), the API and MQTT stay in °C, and so
 * do the inputs for curves and limits.
 */
export type TempUnit = 'c' | 'f';

// One value for the whole page: the plugin page, the dashboard tile and the LED pages are separate bundles, each with its own
// copy of this module.
const shared = globalThis as { UNC_TEMP_UNIT?: TempUnit; UNC_TEMP_BOTH?: boolean };

export const setTempUnit = (unit: unknown, both: unknown = false): void => {
    shared.UNC_TEMP_UNIT = unit === 'f' ? 'f' : 'c';
    shared.UNC_TEMP_BOTH = both === true;
};

export const tempUnit = (): TempUnit => (shared.UNC_TEMP_UNIT === 'f' ? 'f' : 'c');

export const tempBoth = (): boolean => shared.UNC_TEMP_BOTH === true;

const signOf = (unit: TempUnit): string => (unit === 'f' ? '°F' : '°C');

/** The sign of the shown unit. */
export const unitSign = (): string => signOf(tempUnit());

/** A temperature in °C in a unit: whole °F for a whole °C, else one decimal; °C passes through as it is. A difference gets no +32. */
function convert(celsius: number, unit: TempUnit, delta: boolean): number {
    if (unit === 'c') return celsius;
    const f = (celsius * 9) / 5 + (delta ? 0 : 32);
    return Number.isInteger(celsius) ? Math.round(f) : Math.round(f * 10) / 10;
}

/** A temperature in °C as the page shows it, as a number. */
export const tempValue = (celsius: number): number => convert(celsius, tempUnit(), false);

/** A difference of temperatures (a margin, an offset) as a number. */
export const tempDelta = (celsius: number): number => convert(celsius, tempUnit(), true);

function show(value: number | null | undefined, delta: boolean, both: boolean): string {
    if (value == null) return '–';
    const unit = tempUnit();
    const main = `${convert(value, unit, delta)} ${signOf(unit)}`;
    if (!both) return main;
    const other: TempUnit = unit === 'f' ? 'c' : 'f';
    return `${main} (${convert(value, other, delta)} ${signOf(other)})`;
}

/** A temperature in °C as text, in the shown unit and, if switched on, with the other one in brackets. */
export const deg = (value: number | null | undefined): string => show(value, false, tempBoth());

/** The same for a difference. */
export const degDelta = (value: number | null | undefined): string => show(value, true, tempBoth());

/** The shown unit only, for places that are narrow (drawings, the dashboard tile, compact LED texts). */
export const degCompact = (value: number | null | undefined): string => show(value, false, false);

/** The same for a difference. */
export const degDeltaCompact = (value: number | null | undefined): string => show(value, true, false);

/** An input that stays in °C, with the °F value beside it when °F is shown or both units are: "90 °C (194 °F)". */
export const celsiusWith = (celsius: number | null | undefined): string =>
    celsius == null ? '–' : tempUnit() === 'f' || tempBoth() ? `${celsius} °C (${convert(celsius, 'f', false)} °F)` : `${celsius} °C`;

/** The same for a difference: "15 °C (27 °F)". */
export const celsiusDeltaWith = (celsius: number): string =>
    tempUnit() === 'f' || tempBoth() ? `${celsius} °C (${convert(celsius, 'f', true)} °F)` : `${celsius} °C`;

export function tempClass(temp: number | null | undefined, kind: TempKind): TempLevel {
    if (temp == null) return '';
    const [warm, hot] = LIMITS[kind];
    return temp >= hot ? 'hot' : temp >= warm ? 'warm' : 'ok';
}

export const chipName = (name: unknown): string =>
    String(name || '')
        .replace('IT8613', 'ITE IT8613E')
        .replace('IT5571', 'ITE IT5571');

export function fmtSize(bytes: number | null | undefined): string {
    if (!bytes) return '';
    const tb = bytes / 1e12;
    if (tb >= 1) return `${tb >= 10 ? Math.round(tb) : Math.round(tb * 10) / 10} TB`;
    return `${Math.round(bytes / 1e9)} GB`;
}

export const fmtRam = (mb: number): string => (mb >= 1024 ? `${Math.round(mb / 1024)} GB` : `${mb} MB`);

export const speedText = (mbit: number): string => (mbit >= 1000 ? `${mbit / 1000} Gbit/s` : `${mbit} Mbit/s`);

/** "12th Gen Intel(R) Core(TM) i5-1235U @ 2.50GHz" -> "Core i5-1235U" */
export const shortCpu = (name: unknown): string =>
    String(name || '')
        .replace(/\(R\)|\(TM\)|CPU|@.*$|^\d+th Gen\s+Intel\s*/gi, '')
        .replace(/\s+/g, ' ')
        .trim();

/** dmidecode gives MM/DD/YYYY. */
export const biosDate = (s: unknown): string => String(s || '').replace(/^(\d{2})\/(\d{2})\/(\d{4})$/, '$2.$1.$3');

export const friendly = (value: unknown): string =>
    String(value || '')
        .replace(/_/g, ' ')
        .replace(/\s+/g, ' ')
        .trim();

/** Always day.month.year, 24 h, whatever the UI language, in the browser's time zone (or `timeZone`): "27.10.2026 23:41". */
export const dateTime = (seconds: number, timeZone?: string): string =>
    new Date(seconds * 1000)
        .toLocaleString('de-DE', {
            timeZone,
            hour12: false,
            day: '2-digit',
            month: '2-digit',
            year: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        })
        .replace(',', '');

export const rangeFill = (value: number, min: number, max: number): string => `${((value - min) / (max - min)) * 100}%`;

/** "2.5G", "1G", "100M": the negotiated link speed next to the LAN LED. */
export const speedShort = (mbps: number): string => (mbps >= 1000 ? `${mbps / 1000}G` : mbps > 0 ? `${mbps}M` : '');

/** "2,5 Gbit/s" (decimal comma unless `dot`), "100 Mbit/s". */
export const linkSpeed = (mbps: number, dot: boolean): string =>
    mbps >= 1000 ? `${String(mbps / 1000).replace('.', dot ? '.' : ',')} Gbit/s` : `${mbps} Mbit/s`;
