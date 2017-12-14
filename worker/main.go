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
	"log"
	"os"
	"time"

	"github.com/satori/go.uuid"

	"github.com/MorpheoOrg/morpheo-go-packages/client"
	"github.com/MorpheoOrg/morpheo-go-packages/common"
)

func main() {
	conf := NewConsumerConfig()

	// Let's connect with Storage (or use our mock if no storage host was provided)
	var storageBackend client.Storage
	// if conf.StorageHost != "" {
	storageBackend = &client.StorageAPI{
		Hostname: conf.StorageHost,
		Port:     conf.StoragePort,
		User:     conf.StorageUser,
		Password: conf.StoragePassword,
	}
	// } else {
	// 	storageBackend = client.NewStorageAPIMock()
	// }

	// Let's create our peer client to request the blockchain
	peer, err := client.NewPeerAPI(
		"secrets/config.yaml",
		"Aphp",
		"mychannel",
		"mycc",
	)
	if err != nil {
		log.Panicf("Error creating peer client: %s", err)
	}

	// Let's hook to our container backend and create a Worker instance containing
	// our message handlers
	containerRuntime, err := common.NewDockerRuntime(conf.DockerTimeout)
	if err != nil {
		log.Panicf("[FATAL ERROR] Impossible to connect to Docker container backend: %s", err)
	}

	worker := &Worker{
		ID: uuid.NewV4(),
		// Root folder for train/test/predict data (should shared with the container runtime)
		dataFolder: "/data",
		// Subfolder names
		trainFolder:          "train",
		testFolder:           "test",
		predFolder:           "pred",
		perfFolder:           "perf",
		untargetedTestFolder: "untargeted_test",
		modelFolder:          "model",
		// Container runtime image name prefixes
		problemImagePrefix: "problem",
		algoImagePrefix:    "algo",
		// Dependency injection is done here :)
		containerRuntime: containerRuntime,
		storage:          storageBackend,
		peer:             peer,
	}

	// Let's hook with our consumer
	consumer := common.NewNSQConsumer(
		conf.NsqlookupdURLs,
		conf.NsqdURL,
		"compute",
		5*time.Second,
		log.New(os.Stdout, "[NSQ]", log.LstdFlags),
	)

	// Wire our message handlers
	consumer.AddHandler(common.TrainTopic, worker.HandleLearn, conf.LearnParallelism, conf.LearnTimeout)
	consumer.AddHandler(common.PredictTopic, worker.HandlePred, conf.PredictParallelism, conf.PredictTimeout)

	if err != nil {
		log.Panicln(err)
	}
	// Let's connect to the for real and start pulling tasks
	consumer.ConsumeUntilKilled()

	log.Println("[INFO] Consumer has been gracefully stopped... Bye bye!")
	return
}
