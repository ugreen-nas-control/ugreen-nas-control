import type { Model } from './schema';

const BY = 'alexanderschubert (maintainer, issue 302)';
const REF = 'https://github.com/ugreen-nas-control/ugreen-nas-control/issues/302';

export default {
    id: 'dxp6800-pro',
    name: 'DXP6800 Pro',
    series: 'DXP',
    dmiPattern: '^DXP6800 Pro$',
    fixtureDmi: 'DXP6800 Pro',
    cpu: 'Intel Core i5-1235U',
    ram: { type: 'DDR5', mt: 4800 },
    bays: 6,
    m2Slots: 2,
    m2: { slots: ['m2-1', 'm2-2', 'system'], pcieSlot: true },
    lan: { maxMbit: [10000, 10000] },
    led: 'bays',
    caseView: 'full',
    caseVariant: 'rear6800',
    fan: {
        chip: 'ITE IT8613E',
        fans: { fan2: 'cpu', fan3: 'case', fan4: 'case' },
        watchdog: {
            status: 'expected',
            note: 'Expected, from the vendor driver and the kernel it87_wdt: the plugin only feeds a timer it sees counting, never arms it, and yields to it87_wdt. Not confirmed on this hardware.'
        }
    },
    verified: {
        leds: {
            status: 'verified',
            by: BY,
            ref: REF,
            version: '5.6.0',
            date: '2026-11-12',
            note: 'LED control confirmed on 5.6.0 by the maintainer on the development box.'
        },
        fans: {
            status: 'verified',
            by: BY,
            ref: REF,
            version: '5.6.0',
            date: '2026-11-12',
            note: 'Fan control confirmed on 5.6.0; fan2, fan3 and fan4 follow their targets.'
        },
        case: {
            status: 'verified',
            by: BY,
            ref: REF,
            version: '5.6.0',
            date: '2026-11-12',
            note: 'Inside view (M.2 slots on CPU and chipset, system slot, PCIe slot) and the rear drawn after the device.'
        },
        sensors: { status: 'verified', by: BY, ref: REF, version: '5.6.0', date: '2026-11-12' },
        panel: {
            status: 'expected',
            by: BY,
            ref: REF,
            note: 'Front panel reported working on 5.6.0; held power-button detection has not been confirmed.'
        }
    },
    sources: ['https://wundertech.net/best-ugreen-nas/'],
    note: 'Six bays, two M.2 slots plus the internal system slot, a PCIe slot, two 10G LAN ports. Fan control through the IT8613E; the reference model of the plugin.',
    shots: true
} satisfies Model;
