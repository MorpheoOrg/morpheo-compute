package main_test

import (
	"encoding/json"
	// "fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	//"time"

	"github.com/stretchr/testify/assert"

	. "github.com/MorpheoOrg/morpheo-compute/worker"
	"github.com/MorpheoOrg/morpheo-go-packages/client"
	"github.com/MorpheoOrg/morpheo-go-packages/common"
)

var (
	worker      *Worker
	fixtures    *common.Fixtures
	tmpPathData string
)

func TestMain(m *testing.M) {
	// Let's hook to our container mock
	containerRuntime := common.NewMockRuntime()

	// Create storage Mock
	storageMock, err := client.NewStorageAPIMock()
	if err != nil {
		log.Panicln("Error loading Storage Mock: ", err)
	}

	// Load the fixtures
	fixtures, err = common.LoadFixtures()
	if err != nil {
		log.Panicln("Error loading Fixtures: ", err)
	}

	// Let's finally create our worker
	tmpPathData = filepath.Join(os.TempDir(), "data")
	worker = NewWorker(
		tmpPathData, "train", "test", "untargeted_test", "pred", "perf", "model",
		"problem", "algo", containerRuntime,
		storageMock, client.NewOrchestratorAPIMock(),
	)

	// Run the tests
	exitcode := m.Run()

	// Wipe out everything once the tests are done/failed
	if err := os.RemoveAll(tmpPathData); err != nil {
		log.Println(err)
	}

	os.Exit(exitcode)
}

func TestHandleLearn(t *testing.T) {
	// t.Parallel()

	// Pre-setup directory structure for Learn to avoid permission issues
	taskDataFolder := filepath.Join(tmpPathData, fixtures.Orchestrator.Learnuplet[0].Algo.String())
	assert.Nil(t, worker.SetupDirectories(taskDataFolder, 0777))

	// Copy performance.json in tmp
	tmpPathPerfFile := filepath.Join(taskDataFolder, "perf/performance.json")
	pathPerfFile := filepath.Join(common.PathFixturesData["perf"], "performance.json")
	assert.Nil(t, os.Link(pathPerfFile, tmpPathPerfFile))

	// Test the whole pipeline works...
	learnuplet := fixtures.Orchestrator.Learnuplet[0]
	msg, _ := json.Marshal(learnuplet)
	assert.Nil(t, worker.HandleLearn(msg))
}

func TestHandlePred(t *testing.T) {
	// t.Parallel()

	// Pre-setup directory structure for Pred to avoid permission issues
	taskDataFolder := filepath.Join(tmpPathData, fixtures.Orchestrator.Preduplet[0].Model.String())
	assert.Nil(t, worker.SetupDirectories(taskDataFolder, 0777))

	// Copy model predictions in tmp
	modelID := fixtures.Orchestrator.Preduplet[0].Data.String()
	tmpPathPerfFile := filepath.Join(taskDataFolder, filepath.Join("test/pred/", modelID))
	pathPerfFile := filepath.Join(common.PathFixturesData["pred"], modelID)
	assert.Nil(t, os.Link(pathPerfFile, tmpPathPerfFile))

	// Test the whole pipeline works
	preduplet := fixtures.Orchestrator.Preduplet[0]
	msg, _ := json.Marshal(preduplet)
	assert.Nil(t, worker.HandlePred(msg))
}
