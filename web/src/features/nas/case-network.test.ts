import { expect, test } from 'bun:test';
import { caseNetworkPorts } from './case-network';

const port = (name: string, max_mbit: number) => ({ name, max_mbit, speed_mbit: 1000, link: true });

test('a lone 2.5G NIC does not light the missing 10G socket (#323)', () => {
    const nic = port('eth0', 2500);
    expect(caseNetworkPorts([nic], [10000, 2500])).toEqual([undefined, nic]);
});

test('physical sockets follow capability rather than interface names or negotiated speed', () => {
    const slow = port('eth0', 2500);
    const fast = port('eth1', 10000);
    expect(caseNetworkPorts([slow, fast], [10000, 2500])).toEqual([fast, slow]);
    expect(caseNetworkPorts([fast], [10000, 2500])).toEqual([fast, undefined]);
});

test('unknown capabilities fill remaining sockets without duplicating a NIC', () => {
    const unknown = port('eth0', 0);
    const slow = port('eth1', 2500);
    expect(caseNetworkPorts([unknown, slow], [10000, 2500])).toEqual([unknown, slow]);
    expect(caseNetworkPorts([unknown], [2500, 2500])).toEqual([unknown, undefined]);
    expect(caseNetworkPorts([], [10000, 2500])).toEqual([undefined, undefined]);
});
