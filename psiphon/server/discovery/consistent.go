/*
 * Copyright (c) 2024, Psiphon Inc.
 * All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package discovery

import (
	"net"
	"sync"

	"github.com/Psiphon-Labs/consistent"
	"github.com/toidicakhia/psiphon-tunnel/psiphon/server/psinet"
	"github.com/cespare/xxhash"
)

type hasher struct{}

// consistent.Hasher implementation.
func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

type consistentHashingDiscovery struct {
	clk    clock
	config *consistent.Config
	ring   *consistent.Consistent

	sync.RWMutex
}

func NewConsistentHashingDiscovery() (*consistentHashingDiscovery, error) {
	return newConsistentHashingDiscovery(realClock{})
}

func newConsistentHashingDiscovery(clk clock) (*consistentHashingDiscovery, error) {
	return &consistentHashingDiscovery{
		clk: clk,
		config: &consistent.Config{
			PartitionCount:    0, // set in serversChanged
			ReplicationFactor: 1, // ensure all servers are discoverable
			Load:              1, // ensure all servers are discoverable
			Hasher:            hasher{},
		},
	}, nil
}

func (c *consistentHashingDiscovery) serversChanged(newServers []*psinet.DiscoveryServer) {
	if len(newServers) == 0 {
		c.RWMutex.Lock()
		c.ring = nil
		c.RWMutex.Unlock()
	} else {

		members := make([]consistent.Member, len(newServers))
		for i, server := range newServers {
			members[i] = server
		}

		// Note: requires full reinitialization because we cannot change
		// PartitionCount on the fly. Add/Remove do not update PartitionCount
		// and updating ParitionCount is required to ensure that there is not
		// a panic in the Psiphon-Labs/consistent package and that all servers
		// are discoverable.
		c.config.PartitionCount = len(newServers)

		c.RWMutex.Lock()
		c.ring = consistent.New(members, *c.config)
		c.RWMutex.Unlock()
	}
}

func (c *consistentHashingDiscovery) selectServers(clientIP net.IP) []*psinet.DiscoveryServer {

	c.RWMutex.RLock()
	defer c.RWMutex.RUnlock()

	if c.ring == nil {
		// No discoverable servers.
		return nil
	}

	member := c.ring.LocateKey(clientIP)
	if member == nil {
		// Should never happen.
		return nil
	}

	server := member.(*psinet.DiscoveryServer)

	discoveryDate := c.clk.Now()

	// Double check that server is discoverable at this time.
	if discoveryDate.Before(server.DiscoveryDateRange[0]) ||
		!discoveryDate.Before(server.DiscoveryDateRange[1]) {
		return nil
	}

	return []*psinet.DiscoveryServer{server}
}
