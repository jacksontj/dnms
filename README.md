# Distributed Network Monitoring System

## objective
Black-box test the network from the edges/leafs of the network

## How?
Effectively a distributed ping + traceroute across the whole infrastructure at some
interval. This data is then aggregated into a central setvice to expose the data
through an API

## Terms
Peer: another server on the network

NetworkGraph: graph of the entire network
Route: a set of links
NetworkNode: Router in the network-- something that should respond to traceroute
Link: specific connection between 2 NetworkNodes

Ping: a ping with a specific source port
PingGroup: a group of pings against a specific destination

Traceroute: a traceroute with a specific source port
Traceroute Group: a group of traceroutes against a specific destination

## how those fit together?
Any given node will know about the peers in the network. It will intermittently
ping and traceroute peers in the network, and keep track of failures. In the
event of a failure we'll determine what links are at fault for the disruption.


## Implementation

The goal here is to create a few layers that in themselves create something useful

Base parts:
* Memberlist: our peers on the network to talk to
* Mapper: responsible for mapping the network based on who is in the memberlist
* Pinger: ping all peers in the network-- specifically to hit all routes in the mapper
* Aggregator: aggregate all the graph info from the members of the memberlist

## Running

```sh
go build ./...
./dnms -gossipAddr <advertise-ip> -peer <seed-peer:33434>
```

Flags:

| Flag           | Default   | Notes                                                                          |
|----------------|-----------|--------------------------------------------------------------------------------|
| `-gossipAddr`  | local IP  | Address advertised to the memberlist gossip layer.                             |
| `-peer`        | (none)    | Seed `host:port` to join an existing cluster. Empty starts a standalone node.  |
| `-aggregator`  | false     | Run the aggregator HTTP API in addition to the mapper.                         |
| `-httpAddr`    | `:12345`  | Bind address for the HTTP API (graph, routemap, events).                       |
| `-pingPort`    | `33435`   | UDP port for the ping/ack transport. Must be the same on every cluster node.   |
| `-mapInterval` | `1s`      | Pause between traceroutes inside a single source-port sweep.                   |
| `-mapSrcPortStart` / `-mapSrcPortEnd` | auto | Source-port range used to elicit ECMP variation. Defaults to `pingPort+1 .. +11`; main fatals at startup if the range overlaps `-pingPort`. |
| `-pingPeerInterval`  | `100ms` | Pause between successive peers in a ping sweep.                          |
| `-pingRouteInterval` | `1s`    | Pause between successive routes when pinging one peer.                   |
| `-pingTimeout`       | `1s`    | Ack timeout for a single ping.                                           |
| `-metricRingSize`    | `100`   | Number of recent ping samples retained per route.                        |
| `-httpAllowOrigin`   | `*`     | Value for `Access-Control-Allow-Origin`. Lock down by setting to a specific origin. |
| `-httpToken`         | (none)  | If non-empty, requires `Authorization: Bearer <token>` on every HTTP API request. |
| `-aggregatorToken`   | (none)  | Token the aggregator sends when subscribing to peers; defaults to `-httpToken`. |

Note: gossip still uses UDP/TCP `33434` (memberlist), and `-pingPort` is a
*separate* socket that dnms owns directly — no more piggy-backing pings on the
memberlist receive loop.

## HTTP API

All endpoints return JSON. Useful ones:

| Path                          | Description                                                         |
|-------------------------------|---------------------------------------------------------------------|
| `/v1/graph`                   | Full per-node graph (nodes + edges + routes).                       |
| `/v1/graph/nodes`             | Just the nodes the mapper has observed.                             |
| `/v1/graph/edges`             | Just the links.                                                     |
| `/v1/graph/edges/health`      | Per-link fault attribution — see below.                             |
| `/v1/graph/routes`            | Routes with per-route loss/latency/jitter metrics.                  |
| `/v1/mapper/peers`            | Peers the local node knows about.                                   |
| `/v1/mapper/routemap`         | `(src,dst)` → route lookup.                                         |
| `/v1/events/graph`            | Server-Sent Events stream of graph mutations.                       |
| `/v1/aggregator/graph*`       | Same shapes as `/v1/graph*`, but over the cluster-wide merged view. |
| `/v1/aggregator/events/graph` | SSE stream of the aggregated graph.                                 |

### Per-link fault attribution: `/v1/graph/edges/health`

Between any two peers there are usually N different paths through the
intermediate network. Each route has its own observed loss rate. To figure out
*which hop* is responsible for the loss, dnms correlates per-route loss
observations with link membership:

```
For each link L:
  AvgLossThrough    = sample-weighted mean lossRate of routes containing L
  AvgLossNotThrough = sample-weighted mean lossRate of routes NOT containing L
  Suspicion         = AvgLossThrough − AvgLossNotThrough
```

The endpoint returns one row per link sorted by `suspicion` descending. A
genuinely bad link surfaces with a high positive score because every route
through it shares its loss while routes avoiding it don't; a healthy link on
otherwise noisy paths lands near zero or negative.

The same correlation runs on the aggregator side at
`/v1/aggregator/graph/edges/health`, where it has access to every peer's
routes — much more evidence than any single mapper sees on its own.
