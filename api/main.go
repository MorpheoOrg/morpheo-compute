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

	"gopkg.in/kataras/iris.v6"
	"gopkg.in/kataras/iris.v6/adaptors/httprouter" // <--- TODO or adaptors/gorillamux
	"gopkg.in/kataras/iris.v6/middleware/logger"

	"github.com/MorpheoOrg/morpheo-go-packages/common"
)

// TODO: write tests for the two main views

// Available HTTP Routes
const (
	rootRoute   = "/"
	healthRoute = "/health"
	learnRoute  = "/learn"
	predRoute   = "/pred"
)

type apiServer struct {
	conf     *ProducerConfig
	producer common.Producer
}

func (s *apiServer) configureRoutes(app *iris.Framework) {
	app.Get(rootRoute, s.index)
	app.Get(healthRoute, s.health)
	app.Post(learnRoute, s.postLearnuplet)
	app.Post(predRoute, s.postPreduplet)
}

func main() {
	// App-specific config (parses CLI flags)
	conf := NewProducerConfig()

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
	default:
		log.Panicf("Unsupported broker (%s). The only available broker is 'nsq'", conf.Broker)
	}

	// Handlers configuration
	api := &apiServer{
		conf:     conf,
		producer: producer,
	}
	api.configureRoutes(app)

	// Main server loop
	if conf.TLSOn() {
		app.ListenTLS(fmt.Sprintf("%s:%d", conf.Hostname, conf.Port), conf.CertFile, conf.KeyFile)
	} else {
		app.Listen(fmt.Sprintf("%s:%d", conf.Hostname, conf.Port))
	}
}

func (s *apiServer) index(c *iris.Context) {
	// TODO: check broker connectivity here
	c.JSON(iris.StatusOK, []string{rootRoute, healthRoute, learnRoute, predRoute})
}

func (s *apiServer) health(c *iris.Context) {
	c.JSON(iris.StatusOK, map[string]string{"status": "ok"})
}

func (s *apiServer) postLearnuplet(c *iris.Context) {
	var learnUplet common.LearnUplet

	// Unserializing the request body
	if err := json.NewDecoder(c.Request.Body).Decode(&learnUplet); err != nil {
		msg := fmt.Sprintf("Error decoding body to JSON: %s", err)
		log.Printf("[INFO] %s", msg)
		c.JSON(iris.StatusBadRequest, common.NewAPIError(msg))
		return
	}

	// Let's check for required arguments presence and validity
	if err := learnUplet.Check(); err != nil {
		msg := fmt.Sprintf("Invalid learn-uplet: %s", err)
		log.Printf("[INFO] %s", msg)
		c.JSON(iris.StatusBadRequest, common.NewAPIError(msg))
		return
	}

	// Let's put our LearnUplet in the right topic so that it gets processed for real
	taskBytes, err := json.Marshal(learnUplet)
	if err != nil {
		msg := fmt.Sprintf("Failed to remarshal JSON learn-uplet after validation: %s", err)
		log.Printf("[ERROR] %s", msg)
		c.JSON(iris.StatusInternalServerError, common.NewAPIError(msg))
		return
	}
	err = s.producer.Push(common.TrainTopic, taskBytes)
	if err != nil {
		msg := fmt.Sprintf("Failed push learn-uplet into broker: %s", err)
		log.Printf("[ERROR] %s", msg)
		c.JSON(iris.StatusInternalServerError, common.NewAPIError(msg))
		return
	}

	// TODO: notify the orchestrator we're starting this learning process (using the Go orchestrator
	// API). We can either do a PATCH the status field or re-PUT the whole learnuplet (since it has
	// already been computed and is stored in variable taskBytes)

	c.JSON(iris.StatusAccepted, map[string]string{"message": "Learn-uplet ingested accordingly"})
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
