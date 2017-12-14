Morpheo: Compute API
====================

The compute API is a simple HTTP API that accepts learning and prediction tasks
(as *learnuplets* and *preduplets*) from the orchestrator (and the orchestrator
only), validates them and puts them in a distributed task queue.
This folder contains the code for the `compute` api only. The code that runs the
`compute` workers lives in the `compute-worker` folder and is documented there
as well.

API Spec
--------

The API is dead simple. It consists in 4 routes, two of them being completely
trivial:
 * `GET /`: lists all the routes
 * `GET /health`: service liveness probe
 * `POST /pred`: post a preduplet to this route
 * `POST /learn`: post a learnuplet to this route

The API expects the pred/learn uplets to be posted as JSON strings. Their
structure is described [here](https://morpheoorg.github.io/morpheo-orchestrator/modules/collections.html).

Key features
------------

* **Cloud Native**: the API is stateless and horizontaly scalable. The chosen
  broker for now is NSQ, which - stateful though it may be - can easily be
  scaled horizontally.
* **Simple & Low Level**: written in Golang, simple, and intented to stay so :)

CLI Arguments
-------------

```
Usage of ./target/compute-api:

  -broker string
    	Broker type to use (only 'nsq' available for now) (default "nsq")
  -broker-host string
    	The address of the NSQ Broker to talk to (default "nsqd")
  -broker-port int
    	The port of the NSQ Broker to talk to (default 4160)
  -cert string
    	The TLS certs to serve to clients (leave blank for no TLS)
  -host string
    	The hostname our server will be listening on (default "0.0.0.0")
  -key string
    	The TLS key used to encrypt connection (leave blank for no TLS)
  -orchestrator value
    	List of endpoints (scheme and port included) for the orchestrators we want to bind to.
  -port int
    	The port our compute API will be listening on (default 8000)
  -storage value
    	List of endpoints (scheme and port included) for the storage nodes to bind to.
```

Maintainers
-----------
* Étienne Lafarge <etienne@rythm.co>
* Max-Pol Le Brun <maxpol _at_ morpheo.co>