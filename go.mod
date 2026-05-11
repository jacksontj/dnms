module github.com/jacksontj/dnms

go 1.22.0

toolchain go1.22.1

require (
	github.com/hashicorp/go-msgpack v0.5.5
	github.com/hashicorp/memberlist v0.5.3
	github.com/jacksontj/eventsource v0.0.0-20160812052304-e6bd8015d912
	github.com/jacksontj/traceroute v0.0.0-20160823051555-faa79dfb2600
	github.com/montanaflynn/stats v0.7.1
	github.com/sirupsen/logrus v1.9.3
)

require (
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/donovanhide/eventsource v0.0.0-20210830082556-c59027999da0 // indirect
	github.com/google/btree v0.0.0-20180813153112-4030bb1f1f0c // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-metrics v0.5.4 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.1.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-sockaddr v1.0.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/miekg/dns v1.1.58 // indirect
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529 // indirect
	golang.org/x/mod v0.22.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/tools v0.29.0 // indirect
)

// The upstream jacksontj/traceroute fork imports the pre-rename
// github.com/Sirupsen/logrus, which collides case-insensitively with
// github.com/sirupsen/logrus. We vendor a copy under internal/traceroute
// with the import patched.
replace github.com/jacksontj/traceroute => ./internal/traceroute
