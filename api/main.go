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
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"gopkg.in/kataras/iris.v6"
	"gopkg.in/kataras/iris.v6/adaptors/httprouter"
	"gopkg.in/kataras/iris.v6/middleware/logger"

	"github.com/MorpheoOrg/morpheo-go-packages/client"
	"github.com/MorpheoOrg/morpheo-go-packages/common"
)

// TODO: write tests for the two main views

// Available HTTP Routes
const (
	RootRoute   = "/"
	HealthRoute = "/health"
)

type apiServer struct {
	conf     *ProducerConfig
	producer common.Producer
	peer     client.Peer
}

func (s *apiServer) configureRoutes(app *iris.Framework) {
	app.Get(RootRoute, s.index)
	app.Get(HealthRoute, s.health)
	app.Get("/query", s.query)   // For test purposes
	app.Get("/invoke", s.invoke) // For test purposes
}

// SetIrisApp sets the base for the Iris App
func (s *apiServer) SetIrisApp() *iris.Framework {
	// Iris setup
	app := iris.New()
	app.Adapt(iris.DevLogger())
	app.Adapt(httprouter.New())

	// Logging middleware configuration
	customLogger := logger.New(logger.Config{
		Status: true,
		IP:     true,
		Method: true,
		Path:   true,
	})
	app.Use(customLogger)

	s.configureRoutes(app)
	return app
}

func main() {
	// App-specific config (parses CLI flags)
	conf := NewProducerConfig()

	// Let's dependency inject the producer for the chosen Broker
	var producer common.Producer
	switch conf.Broker {
	case common.BrokerNSQ:
		var err error
		producer, err = common.NewNSQProducer(conf.BrokerHost, conf.BrokerPort)
		defer producer.Stop()
		if err != nil {
			log.Panicln(err)
		}
	case common.BrokerMOCK:
		producer = &common.ProducerMOCK{}
	default:
		log.Panicf("Unsupported broker (%s). Available brokers: 'nsq', 'mock'", conf.Broker)
	}

	// Let's create our peer client to request the blockchain
	// TODO: WITH ADMIN/USER ID INSTEAD
	peer, err := client.NewPeerAPI(
		"secrets/config.yaml",
		"Aphp",
		"mychannel",
		"mycc",
	)
	if err != nil {
		log.Panicf("Error creating peer client: %s", err)
	}

	// Handlers configuration
	api := &apiServer{
		conf:     conf,
		producer: producer,
		peer:     peer,
	}

	app := api.SetIrisApp()

	go api.relayNewLearnuplet()

	// Main server loop
	if conf.TLSOn() {
		app.ListenTLS(fmt.Sprintf("%s:%d", conf.Hostname, conf.Port), conf.CertFile, conf.KeyFile)
	} else {
		app.Listen(fmt.Sprintf("%s:%d", conf.Hostname, conf.Port))
	}
}

func (s *apiServer) index(c *iris.Context) {
	// TODO: check broker connectivity here
	c.JSON(iris.StatusOK, []string{RootRoute, HealthRoute})
}

func (s *apiServer) health(c *iris.Context) {
	c.JSON(iris.StatusOK, map[string]string{"status": "ok"})
}

func (s *apiServer) postLearnuplet(learnuplet common.Learnuplet) error {
	// Let's check for required arguments presence and validity
	if err := learnuplet.Check(); err != nil {
		return fmt.Errorf("[ERROR] Invalid learnuplet: %s", err)
	}

	// Let's put our Learnuplet in the right topic so that it gets processed for real
	taskBytes, err := json.Marshal(learnuplet)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to remarshal JSON learnuplet after validation: %s", err)
	}

	err = s.producer.Push(common.TrainTopic, taskBytes)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed push learn-uplet into broker: %s", err)
	}
	return nil
}

func (s *apiServer) postPreduplet(c *iris.Context) {
	var predUplet common.Preduplet

	// Unserializing the request body
	if err := json.NewDecoder(c.Request.Body).Decode(&predUplet); err != nil {
		msg := fmt.Sprintf("Error decoding body to JSON: %s", err)
		log.Printf("[INFO] %s", msg)
		c.JSON(iris.StatusBadRequest, common.NewAPIError(msg))
		return
	}

	// Let's check for required arguments presence and validity
	if err := predUplet.Check(); err != nil {
		msg := fmt.Sprintf("Invalid pred-uplet: %s", err)
		log.Printf("[INFO] %s", msg)
		c.JSON(iris.StatusBadRequest, common.NewAPIError(msg))
		return
	}

	taskBytes, err := json.Marshal(predUplet)
	if err != nil {
		msg := fmt.Sprintf("Failed to remarshal preduplet to JSON: %s", err)
		log.Printf("[ERROR] %s", msg)
		c.JSON(iris.StatusInternalServerError, common.NewAPIError(msg))
		return
	}
	err = s.producer.Push(common.PredictTopic, taskBytes)
	if err != nil {
		msg := fmt.Sprintf("Failed to push preduplet task into broker: %s", err)
		log.Printf("[ERROR] %s", msg)
		c.JSON(iris.StatusInternalServerError, common.NewAPIError(msg))
		return
	}

	// TODO: notify the orchestrator we're starting this learning process (using the Go orchestrator
	// API). We can either do a PATCH the status field or re-PUT the whole learnuplet (since it has
	// already been computed and is stored in variable taskBytes)

	c.JSON(iris.StatusAccepted, map[string]string{"message": "Pred-uplet ingested"})
}

// ================================================================================
// Go routine that pings the blockchain
// ================================================================================
// Note that this might be changed after a while if a successfull event listenner
// can be easily plugged to the blockchain.
// The code is not clean

func (s *apiServer) relayNewLearnuplet() {
	// brokerLearnQueue represents the learnuplet(s) that have been posted to the
	// broker, but are still with a status "todo".
	// Without this poor little slice of string, each learnuplet in the broker queue
	// with a status "todo" would be posted again every 5s... TOFIX this logic
	var brokerLearnQueue []string

	for {
		time.Sleep(5 * time.Second)

		// Retrieve Learnuplets with status "todo" from peer
		learnupletsBytes, err := s.peer.QueryStatusLearnuplet("todo")
		if err != nil {
			log.Printf("[ERROR] Failed to queryStatusLearnuplet: %s", err)
			continue
		}

		// Unmarshal Learnuplets
		var learnupletsChaincode []common.LearnupletChaincode
		err = json.Unmarshal(learnupletsBytes, &learnupletsChaincode)
		if err != nil {
			log.Printf("[ERROR] Failed to Unmarshal learnuplets: %s", err)
			continue
		}
		log.Printf("[INFO] %d learnuplet(s) with status \"todo\" received from peer", len(learnupletsChaincode))
		if len(learnupletsChaincode) == 0 {
			continue
		}

		// Convert them in the Compute format (TEMPORARY)
		var learnuplets []common.Learnuplet
		for _, learnupletChaincode := range learnupletsChaincode {
			learnupletFormat, err := learnupletChaincode.LearnupletFormat()
			if err != nil {
				log.Printf("[ERROR] Failed to format chaincode-%s: %s", learnupletChaincode.Key, err)
				continue
			}
			// Check learnuplet is valid and add it to the list
			err = learnupletFormat.Check()
			if err != nil {
				log.Printf("[ERROR] Invalid %s: %s", learnupletChaincode.Key, err)
				continue
			}
			learnuplets = append(learnuplets, learnupletFormat)
		}
		if len(learnuplets) == 0 {
			continue
		}

		// post the learnuplets if not already done
		var learnupletTodoList []string
		for _, learnuplet := range learnuplets {
			learnupletTodoList = append(learnupletTodoList, learnuplet.Key)
			if stringInSlice(learnuplet.Key, brokerLearnQueue) {
				continue
			}
			log.Printf("[DEBUG] Posting %s to broker", learnuplet.Key)
			err = s.postLearnuplet(learnuplet)
			if err != nil {
				log.Printf("[ERROR] Failed to postLearnuplet: %s", err)
				continue
			}
			brokerLearnQueue = append(brokerLearnQueue, learnuplet.Key)
		}

		// Clean brokerLearnQueue
		log.Printf("[INFO] %d learnuplet(s) already in the broker queue", len(brokerLearnQueue))
		for len(brokerLearnQueue) > 0 {
			if !stringInSlice(brokerLearnQueue[0], learnupletTodoList) {
				brokerLearnQueue = brokerLearnQueue[1:]
				log.Printf("[INFO] %d learnuplet(s) already in the broker queue", len(brokerLearnQueue))
			} else {
				break
			}
		}
	}
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// ================================================================================
// For test purposes only
// ================================================================================

// query allows to query the blockchain via URL PARAMETERS
func (s *apiServer) query(c *iris.Context) {
	// Retrieve and format URL parameters
	queryFcn := c.URLParam("fcn")
	queryArgs := strings.Split(c.URLParam("args"), "|")

	// Query the peer
	query, err := s.peer.Query(queryFcn, queryArgs)
	if err != nil {
		c.JSON(iris.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	showJSON(c, query)
}

// invoke allows to invoke a transaction in the blockchain via URL PARAMETERS
func (s *apiServer) invoke(c *iris.Context) {
	// Retrieve and format URL parameters
	queryFcn := c.URLParam("fcn")
	queryArgs := strings.Split(c.URLParam("args"), "|")

	// Invoke the peer
	id, nonce, err := s.peer.Invoke(queryFcn, queryArgs)
	if err != nil {
		c.JSON(iris.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Display the results
	c.JSON(iris.StatusOK, map[string]string{"id": id, "nonce": string(nonce)})
}

func showJSON(c *iris.Context, bytesJSON []byte) {
	if len(bytesJSON) == 0 {
		c.JSON(iris.StatusInternalServerError, map[string]string{})
		return
	}

	var m []map[string]interface{}
	var m2 map[string]interface{}
	if err := json.Unmarshal(bytesJSON, &m); err != nil {
		if err := json.Unmarshal(bytesJSON, &m2); err != nil {
			msg := fmt.Sprintf("Failed to unmarshal peer response: %s. RAW bytes: %s", err, bytesJSON)
			c.JSON(iris.StatusInternalServerError, map[string]string{"error": msg})
			return
		}
		c.JSON(iris.StatusOK, m2)
		return
	}
	// Display the results
	c.JSON(iris.StatusOK, m)
}
