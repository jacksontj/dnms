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
