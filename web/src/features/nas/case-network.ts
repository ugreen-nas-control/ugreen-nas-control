import type { NetPort } from '../../core/contract';

/** Match physical sockets by supported speed, including when one NIC is missing (#323).
 * Unknown capabilities retain kernel order in the remaining sockets. */
export function caseNetworkPorts(ports: NetPort[], capacities: number[]): (NetPort | undefined)[] {
    const unused = ports.slice();
    const matched = capacities.map(capacity => {
        const index = unused.findIndex(port => port.max_mbit === capacity);
        return index < 0 ? undefined : unused.splice(index, 1)[0];
    });
    const unknown = unused.filter(port => !port.max_mbit);
    return matched.map(port => port ?? unknown.shift());
}
