/*
 * Copyright Morpheo Org. 2017
 *
 * contact@morpheo.co
 *
 * This software is part of the Morpheo project, an open-source machine
 * learning platform.
 *
 * This software is governed by the CeCILL license, compatible with the
 * GNU GPL, under French law and abiding by the rules of distribution of
 * free software. You can  use, modify and/ or redistribute the software
 * under the terms of the CeCILL license as circulated by CEA, CNRS and
 * INRIA at the following URL "http://www.cecill.info".
 *
 * As a counterpart to the access to the source code and  rights to copy,
 * modify and redistribute granted by the license, users are provided only
 * with a limited warranty  and the software's author,  the holder of the
 * economic rights,  and the successive licensors  have only  limited
 * liability.
 *
 * In this respect, the user's attention is drawn to the risks associated
 * with loading,  using,  modifying and/or developing or reproducing the
 * software by the user in light of its specific status of free software,
 * that may mean  that it is complicated to manipulate,  and  that  also
 * therefore means  that it is reserved for developers  and  experienced
 * professionals having in-depth computer knowledge. Users are therefore
 * encouraged to load and test the software's suitability as regards their
 * requirements in conditions enabling the security of their systems and/or
 * data to be ensured and,  more generally, to use and operate it in the
 * same conditions as regards security.
 *
 * The fact that you are presently reading this means that you have had
 * knowledge of the CeCILL license and that you accept its terms.
 */

package main

import (
	"flag"
	"time"

	"github.com/MorpheoOrg/morpheo-go-packages/common"
)

// ConsumerConfig holds the consumer configuration
type ConsumerConfig struct {
	// Broker
	NsqlookupdURLs     []string
	NsqdURL            string
	LearnParallelism   int
	PredictParallelism int
	LearnTimeout       time.Duration
	PredictTimeout     time.Duration

	// Other compute services
	OrchestratorHost     string
	OrchestratorPort     int
	OrchestratorUser     string
	OrchestratorPassword string
	StorageHost          string
	StoragePort          int
	StorageUser          string
	StoragePassword      string

	// Container Runtime
	DockerHost    string
	DockerTimeout time.Duration
}

// NewConsumerConfig parses CLI flags, generates and validates a ConsumerConfig
func NewConsumerConfig() (conf *ConsumerConfig) {
	var (
		nsqlookupdURLs     common.MultiStringFlag
		nsqdURL            string
		learnParallelism   int
		predictParallelism int
		learnTimeout       time.Duration
		predictTimeout     time.Duration

		orchestratorHost     string
		orchestratorPort     int
		orchestratorUser     string
		orchestratorPassword string
		storageHost          string
		storagePort          int
		storageUser          string
		storagePassword      string

		dockerHost    string
		dockerTimeout time.Duration
	)

	// CLI Flags
	flag.Var(&nsqlookupdURLs, "nsqlookupd-urls", "URL(s) of NSQLookupd instances to connect to")
	flag.StringVar(&nsqdURL, "http-address", "nsqd:4151", "URL of NSQd instance to connect to")
	flag.IntVar(&learnParallelism, "learn-parallelism", 1, "Number of learning task that this worker can execute in parallel.")
	flag.IntVar(&predictParallelism, "predict-parallelism", 1, "Number of learning task that this worker can execute in parallel.")
	flag.DurationVar(&learnTimeout, "learn-timeout", 20*time.Minute, "After this delay, learning tasks are timed out (default: 20m)")
	flag.DurationVar(&predictTimeout, "predict-timeout", 20*time.Minute, "After this delay, prediction tasks are timed out (default: 20m)")

	flag.StringVar(&orchestratorHost, "orchestrator-host", "", "Hostname of the orchestrator to send notifications to (leave blank to use the Orchestrator API Mock)")
	flag.IntVar(&orchestratorPort, "orchestrator-port", 80, "TCP port to contact the orchestrator on (default: 80)")
	flag.StringVar(&orchestratorUser, "orchestrator-user", "u", "Basic Authentication username of the orchestrator API")
	flag.StringVar(&orchestratorPassword, "orchestrator-password", "p", "Basic Authentication password of the orchestrator API")

	flag.StringVar(&storageHost, "storage-host", "", "Hostname of the storage API to retrieve data from (leave blank to use the Storage API Mock)")
	flag.IntVar(&storagePort, "storage-port", 80, "TCP port to contact storage on (default: 80)")
	flag.StringVar(&storageUser, "storage-user", "u", "Basic Authentication username of the storage API")
	flag.StringVar(&storagePassword, "storage-password", "p", "Basic Authentication password of the storage API")

	flag.DurationVar(&dockerTimeout, "docker-timeout", 15*time.Minute, "Docker commands timeout (concerns builds, runs, pulls, etc...) (default: 15m)")

	flag.Parse()

	if len(nsqlookupdURLs) == 0 {
		nsqlookupdURLs = append(nsqlookupdURLs, "nsqlookupd:4161")
	}

	return &ConsumerConfig{
		NsqlookupdURLs:     nsqlookupdURLs,
		NsqdURL:            nsqdURL,
		LearnParallelism:   learnParallelism,
		PredictParallelism: predictParallelism,
		LearnTimeout:       learnTimeout,
		PredictTimeout:     predictTimeout,

		// Other compute services
		OrchestratorHost:     orchestratorHost,
		OrchestratorPort:     orchestratorPort,
		OrchestratorUser:     orchestratorUser,
		OrchestratorPassword: orchestratorPassword,
		StorageHost:          storageHost,
		StoragePort:          storagePort,
		StorageUser:          storageUser,
		StoragePassword:      storagePassword,

		// Container Runtime
		DockerHost:    dockerHost,
		DockerTimeout: dockerTimeout,
	}
}
