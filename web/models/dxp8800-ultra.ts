import type { Model } from './schema';

export default {
    id: 'dxp8800-ultra',
    name: 'DXP8800 Ultra',
    series: 'DXP',
    dmiPattern: '^DXP8800 Ultra',
    fixtureDmi: 'DXP8800 Ultra',
    cpu: 'Intel Core 7 150U',
    ram: { type: 'DDR5', mt: 5600 },
    bays: 8,
    m2Slots: 2,
    m2: { slots: ['m2-1', 'm2-2'], pcieSlot: true },
    lan: { maxMbit: [10000, 10000] },
    led: 'bays',
    caseView: 'full',
    caseVariant: 'rear8800',
    fan: null,
    verified: {
        leds: {
            status: 'community',
            by: 'halloleo (issue 293)',
            version: '5.7.0',
            date: '2026-11-16',
            ref: 'https://github.com/ugreen-nas-control/ugreen-nas-control/issues/293',
            note: 'Power colour and bay 2 colour writes confirmed on 5.6.0; Identify selects the correct LEDs for bays 1, 3 and 8 on 5.7.0, including empty bays. An intermittent read stall and a disk3 command without controller confirmation during update remain unresolved.'
        },
        fans: { status: 'unsupported' },
        case: {
            status: 'expected',
            note: "Front and rear after UGREEN's product renders. The inside is drawn after the DXP8800 Plus (photos of the Plus used as proxy), slot positions and numbering assumed."
        },
        sensors: { status: 'expected' },
        panel: { status: 'unsupported', note: "Newer than UGREEN's driver source, which names no button or beeper path for it." }
    },
    sources: [
        'https://ai.ugreen.com/products/ugreen-nasync-dxp8800-ultra-nas-storage',
        'https://nascompares.com/2026/06/22/ugreen-dxp6800-ultra-and-dxp8800-ultra-nas-revealed/',
        'https://github.com/ugreen-nas-control/ugreen-nas-control/issues/293'
    ],
    note: 'Eight bays, two M.2 slots, two 10G LAN ports and two Thunderbolt 4 ports. LED colour control and Identify bay order confirmed by an owner; intermittent controller communication errors remain unresolved. No fan control.',
    shots: true
} satisfies Model;
