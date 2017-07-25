Morpheo: Compute Workers
========================

Compute workers prepare and execute containerized machine learning workflows.

It retrieves tasks from a (distributed) broker, pulls the *problem workflow*
container (that describes how training and prediction tasks are executed and
evaluated) and runs the training/prediction tasks on the network-isolated
*submission* container. Training tasks' performance increase is also evaluated
and sent to the orchestrator.

The specifications of the containers ran by compute is documented
[here](https://morpheoorg.github.io/morpheo/).

Examples *problem workflow* and *submission* containers can be found
[here](https://github.com/MorpheoOrg/hypnogram-wf).

CLI Arguments
-------------

```
Usage of compute-worker:

  -docker-timeout duration
    	Docker commands timeout (concerns builds, runs, pulls, etc...) (default: 15m) (default 15m0s)
  -learn-parallelism int
    	Number of learning task that this worker can execute in parallel. (default 1)
  -learn-timeout duration
    	After this delay, learning tasks are timed out (default: 20m) (default 20m0s)
  -nsqlookupd-urls value
    	URL(s) of NSQLookupd instances to connect to
  -orchestrator-host string
    	Hostname of the orchestrator to send notifications to (leave blank to use the Orchestrator API Mock)
  -orchestrator-password string
    	Basic Authentication password of the orchestrator API (default "p")
  -orchestrator-port int
    	TCP port to contact the orchestrator on (default: 80) (default 80)
  -orchestrator-user string
    	Basic Authentication username of the orchestrator API (default "u")
  -predict-parallelism int
    	Number of learning task that this worker can execute in parallel. (default 1)
  -predict-timeout duration
    	After this delay, prediction tasks are timed out (default: 20m) (default 20m0s)
  -storage-host string
    	Hostname of the storage API to retrieve data from (leave blank to use the Storage API Mock)
  -storage-password string
    	Basic Authentication password of the storage API (default "p")
  -storage-port int
    	TCP port to contact storage on (default: 80) (default 80)
  -storage-user string
    	Basic Authentication username of the storage API (default "u")

```

### TODO

* Retry policies for our tasks depending on the source of the error

Maintainers
-----------
* Étienne Lafarge <etienne@rythm.co>
