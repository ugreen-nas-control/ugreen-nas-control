import type { Model } from './schema';

export default {
    id: 'dx4600-variants',
    name: 'DX4600 variants',
    series: 'DX',
    dmiPattern: '^DX4600([+]| Plus)',
    fixtureDmi: 'DX4600+',
    cpu: null,
    ram: null,
    bays: 4,
    m2Slots: null,
    m2: null,
    lan: null,
    led: 'bays',
    caseView: 'front-rear',
    caseVariant: 'rear4600',
    fan: null,
    verified: {
        leds: {
            status: 'expected',
            ref: 'https://github.com/miskcoo/ugreen_leds_controller/pull/126',
            note: 'Keep recognition of other DX4600-family names, including DX4600+, without assigning the Pro CPU or other specifications to them. Upstream PR 126 includes DX4600+ in its legacy protocol discovery path but reports hardware testing only on the plain DX4600. This variant and its bay order remain unconfirmed with this plugin.'
        },
        fans: { status: 'unsupported' },
        case: {
            status: 'expected',
            note: 'Front and rear use the existing family drawing. No inside view: RAM and M.2 layout are unknown for these variants.'
        },
        sensors: { status: 'expected' },
        panel: { status: 'expected', note: 'DX4600-family driver data only; not confirmed on these variants.' }
    },
    sources: ['https://github.com/miskcoo/ugreen_leds_controller/pull/126'],
    note: 'Other DX4600-family variants: specifications and physical layout remain unconfirmed. No fan control.',
    shots: false,
    fixture: { cpu: 'Intel Pentium Silver N6005', ram: { type: 'DDR4', mt: 3200 } }
} satisfies Model;
