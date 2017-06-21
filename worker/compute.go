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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/MorpheoOrg/go-packages/client"
	"github.com/MorpheoOrg/go-packages/common"
)

// Worker describes a worker (where it stores its data, which container runtime it uses...).
// Most importantly, it carefully implements all the steps of our learning/testing/prediction
// workflow.
//
// For an in-detail understanding of what these different steps do and how, check out Camille's
// awesome example: https://github.com/MorpheoOrg/hypnogram-wf
// The doc also gets there in detail: https://morpheoorg.github.io/morpheo/modules/learning.html
type Worker struct {
	// Worker configuration variables
	dataFolder           string
	trainFolder          string
	testFolder           string
	untargetedTestFolder string
	modelFolder          string
	predFolder           string
	perfFolder           string
	problemImagePrefix   string
	algoImagePrefix      string

	// ContainerRuntime abstractions
	containerRuntime common.ContainerRuntime

	// Morpheo API clients
	storage      client.Storage
	orchestrator client.Orchestrator
}

// NewWorker creates a Worker instance
func NewWorker(dataFolder, trainFolder, testFolder, untargetedTestFolder, predFolder, perfFolder, modelFolder, problemImagePrefix, algoImagePrefix string, containerRuntime common.ContainerRuntime, storage client.Storage, orchestrator client.Orchestrator) *Worker {
	return &Worker{
		dataFolder:           dataFolder,
		trainFolder:          trainFolder,
		testFolder:           testFolder,
		predFolder:           predFolder,
		perfFolder:           perfFolder,
		untargetedTestFolder: untargetedTestFolder,
		modelFolder:          modelFolder,
		problemImagePrefix:   problemImagePrefix,
		algoImagePrefix:      algoImagePrefix,

		containerRuntime: containerRuntime,
		storage:          storage,
		orchestrator:     orchestrator,
	}
}

// HandleLearn manages a learning task (orchestrator status updates, etc...)
func (w *Worker) HandleLearn(message []byte) (err error) {
	log.Println("[DEBUG] Starting learning task")

	// Unmarshal the learn-uplet
	var task common.LearnUplet

	err = json.NewDecoder(bytes.NewReader(message)).Decode(&task)
	if err != nil {
		return fmt.Errorf("Error un-marshaling learn-uplet: %s -- Body: %s", err, message)
	}

	if err = task.Check(); err != nil {
		return fmt.Errorf("Error in train task: %s -- Body: %s", err, message)
	}

	// Update its status to pending on the orchestrator
	err = w.orchestrator.UpdateUpletStatus(common.TypeLearnUplet, common.TaskStatusPending, task.ID, task.WorkerID)
	if err != nil {
		return fmt.Errorf("Error seting learnuplet status to pending on the orchestrator: %s", err)
	}

	err = w.LearnWorkflow(task)
	if err != nil {
		log.Println(err)
		return err
		// TODO: handle fatal and non-fatal errors differently and set learnuplet status to failed only
		// if the error was fatal
		err = w.orchestrator.UpdateUpletStatus(common.TypeLearnUplet, common.TaskStatusFailed, task.ID, task.WorkerID)
		if err != nil {
			return fmt.Errorf("Error setting learnuplet status to failed on the orchestrator: %s", err)
		}
		return fmt.Errorf("Error in LearnWorkflow: %s", err)
	}

	return nil
}

// LearnWorkflow implements our learning workflow
func (w *Worker) LearnWorkflow(task common.LearnUplet) (err error) {
	log.Println("[DEBUG] Starting learning workflow")

	// Setup directory structure
	taskDataFolder := filepath.Join(w.dataFolder, task.Algo.String())
	trainFolder := filepath.Join(taskDataFolder, w.trainFolder)
	testFolder := filepath.Join(taskDataFolder, w.testFolder)
	untargetedTestFolder := filepath.Join(taskDataFolder, w.untargetedTestFolder)
	modelFolder := filepath.Join(taskDataFolder, w.modelFolder)
	perfFolder := filepath.Join(taskDataFolder, w.perfFolder)

	err = os.MkdirAll(trainFolder, os.ModeDir)
	if err != nil {
		return fmt.Errorf("Error creating train folder under %s: %s", trainFolder, err)
	}
	err = os.MkdirAll(testFolder, os.ModeDir)
	if err != nil {
		return fmt.Errorf("Error creating test folder under %s: %s", testFolder, err)
	}
	err = os.MkdirAll(untargetedTestFolder, os.ModeDir)
	if err != nil {
		return fmt.Errorf("Error creating untargeted test folder under %s: %s", untargetedTestFolder, err)
	}
	err = os.MkdirAll(modelFolder, os.ModeDir)
	if err != nil {
		return fmt.Errorf("Error creating model folder under %s: %s", untargetedTestFolder, err)
	}

	// Let's make sure these folders are wiped out once the task is done/failed
	defer os.RemoveAll(taskDataFolder)

	// Load problem workflow
	problemWorkflow, err := w.storage.GetProblemWorkflowBlob(task.Workflow)
	if err != nil {
		return fmt.Errorf("Error pulling problem workflow %s from storage: %s", task.Workflow, err)
	}

	problemImageName := fmt.Sprintf("%s-%s", w.problemImagePrefix, task.Workflow)
	err = w.ImageLoad(problemImageName, problemWorkflow)
	if err != nil {
		return fmt.Errorf("Error loading problem workflow image %s in Docker daemon: %s", task.Workflow, err)
	}
	problemWorkflow.Close()
	defer w.containerRuntime.ImageUnload(problemImageName)

	// Load algo
	algo, err := w.storage.GetAlgoBlob(task.Algo)
	if err != nil {
		return fmt.Errorf("Error pulling algo %s from storage: %s", task.Algo, err)
	}

	algoImageName := fmt.Sprintf("%s-%s", w.algoImagePrefix, task.Algo)
	err = w.ImageLoad(algoImageName, algo)
	if err != nil {
		return fmt.Errorf("Error loading algo image %s in Docker daemon: %s", algoImageName, err)
	}
	algo.Close()
	defer w.containerRuntime.ImageUnload(algoImageName)

	// Pull model if a model_start parameter was given in the learn-uplet
	if task.Rank > 0 {
		model, err := w.storage.GetModelBlob(task.ModelStart)
		if err != nil {
			return fmt.Errorf("Error pulling start model %s from storage: %s", task.ModelStart, err)
		}

		err = w.UntargzInFolder(modelFolder, model)
		if err != nil {
			return fmt.Errorf("Error un-tar-gz-ing model: %s", err)
		}
		model.Close()
	}

	// Pulling train dataset
	for _, dataID := range task.TrainData {
		data, err := w.storage.GetDataBlob(dataID)
		if err != nil {
			return fmt.Errorf("Error pulling train dataset %s from storage: %s", dataID, err)
		}
		path := fmt.Sprintf("%s/%s", trainFolder, dataID)
		dataFile, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("Error creating file %s: %s", path, err)
		}
		n, err := io.Copy(dataFile, data)
		if err != nil {
			return fmt.Errorf("Error copying train data file %s (%d bytes written): %s", path, n, err)
		}
		dataFile.Close()
		data.Close()
	}

	// And the test data
	for _, dataID := range task.TestData {
		data, err := w.storage.GetDataBlob(dataID)
		if err != nil {
			return fmt.Errorf("Error pulling test dataset %s from storage: %s", dataID, err)
		}
		path := fmt.Sprintf("%s/%s", testFolder, dataID)
		dataFile, err := os.Create(path)
		n, err := io.Copy(dataFile, data)
		if err != nil {
			return fmt.Errorf("Error copying test data file %s (%d bytes written): %s", path, n, err)
		}
		dataFile.Close()
		data.Close()
	}

	// Let's remove targets from the test data
	_, err = w.UntargetTestingVolume(problemImageName, testFolder, untargetedTestFolder)
	if err != nil {
		return fmt.Errorf("Error preparing problem %s for %s: %s", task.Workflow, task.ModelStart, err)
	}

	// Let's pass the task to our execution backend, now that everything should be in place
	_, err = w.Train(algoImageName, trainFolder, untargetedTestFolder, modelFolder)
	if err != nil {
		return fmt.Errorf("Error in train task: %s -- Body: %s", err, task)
	}

	// Let's compute the performance !
	_, err = w.ComputePerf(problemImageName, trainFolder, testFolder, untargetedTestFolder, perfFolder)
	if err != nil {
		// FIXME: do not return here
		return fmt.Errorf("Error computing perf for problem %s and model (new) %s: %s", task.Workflow, task.ModelEnd, err)
	}

	// Let's create a new model and post it to storage
	algoInfo, err := w.storage.GetAlgo(task.Algo)
	if err != nil {
		return fmt.Errorf("Error retrieving algorithm %s metadata: %s", task.Algo, err)
	}
	newModel := common.NewModel(task.ModelEnd, algoInfo)
	newModel.ID = task.ModelEnd

	// Let's compress our model in a separate goroutine while writing it on disk on the fly
	path := fmt.Sprintf("%s/model.tar.gz", taskDataFolder)
	modelArchiveWriter, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("Error creating new model archive file %s: %s", path, err)
	}
	err = w.TargzFolder(modelFolder, modelArchiveWriter)
	if err != nil {
		return fmt.Errorf("Error tar-gzipping new model %s: %s", task.ModelEnd, err)
	}
	modelArchiveWriter.Close()

	modelArchiveReader, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Error reading new model archive file %s: %s", path, err)
	}
	modelArchiveStat, err := modelArchiveReader.Stat()
	if err != nil {
		return fmt.Errorf("Error reading new model archive size %s: %s", path, err)
	}

	err = w.storage.PostModel(newModel, modelArchiveReader, modelArchiveStat.Size())
	if err != nil {
		return fmt.Errorf("Error streaming new model %s to storage: %s", task.ModelEnd, err)
	}
	modelArchiveReader.Close()

	// Let's send the perf file to the orchestrator
	performanceFilePath := fmt.Sprintf("%s/performance.json", perfFolder)
	log.Println(performanceFilePath)
	resultFile, err := os.Open(performanceFilePath)
	if err != nil {
		return fmt.Errorf("Error reading performance file %s: %s", performanceFilePath, err)
	}
	perfuplet := client.Perfuplet{}
	err = json.NewDecoder(resultFile).Decode(&perfuplet)
	if err != nil {
		return fmt.Errorf("Error un-marshaling performance file to JSON: %s", err)
	}
	perfuplet.Status = common.TaskStatusDone

	err = w.orchestrator.PostLearnResult(task.ID, perfuplet)
	if err != nil {
		return fmt.Errorf("Error posting learn result %s to orchestrator: %s", task.ModelEnd, err)
	}

	resultFile.Close()
	os.Remove(performanceFilePath)

	log.Printf("[INFO] Train finished with success, cleaning up...")

	return
}

// HandlePred handles our prediction tasks
// func (w *Worker) HandlePred(message []byte) (err error) {
// 	var task common.Preduplet
// 	err = json.NewDecoder(bytes.NewReader(message)).Decode(&task)
// 	if err != nil {
// 		return fmt.Errorf("Error un-marshaling pred-uplet: %s -- Body: %s", err, message)
// 	}
//
//	_, err = w.Predict(modelImageName, untargetedTestFolder)
//	if err != nil {
//		return fmt.Errorf("Error in test task: %s -- Body: %s", err, task)
//	}
//
// 	// TODO: send the prediction to the viewer, asynchronously
// 	log.Printf("Predicition completed with success. Predicition %s", prediction)
//
// 	return
// }

// ImageLoad loads the docker image corresponding to a problem workflow/submission container in the
// container runtime that will then run this problem workflow/submission container
func (w *Worker) ImageLoad(imageName string, imageReader io.Reader) error {
	imageTarReader, err := gzip.NewReader(imageReader)
	if err != nil {
		return fmt.Errorf("Error un-gzipping image %s: %s", imageName, err)
	}
	defer imageTarReader.Close()

	image, err := w.containerRuntime.ImageBuild(imageName, imageTarReader)
	if err != nil {
		return fmt.Errorf("Error building image %s: %s", imageName, err)
	}
	defer image.Close()

	return w.containerRuntime.ImageLoad(imageName, image)
}

// UntargzInFolder unflattens a .tar.gz archive provided as an io.Reader into a given folder
func (w *Worker) UntargzInFolder(folder string, tarGzReader io.Reader) error {
	zipReader, err := gzip.NewReader(tarGzReader)
	if err != nil {
		return fmt.Errorf("Error un-gzipping model: %s", err)
	}
	defer zipReader.Close()

	tarReader := tar.NewReader(zipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("Error reading model tar archive: %s")
		}

		path := filepath.Join(folder, header.Name)
		info := header.FileInfo()
		if info.IsDir() {
			if err := os.MkdirAll(path, info.Mode()); err != nil {
				return fmt.Errorf("Error unflattening tar archive: error creating directory %s: %s", path, err)
			}
			continue
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return fmt.Errorf("Error unflattening tar archive: error creating file %s: %s", path, err)
		}
		defer file.Close()

		_, err = io.Copy(file, tarReader)
		if err != nil {
			return fmt.Errorf("Error unflattening tar archive: error writing to file %s: %s", path, err)
		}
	}
	return nil
}

// TargzFolder tars and gzips a folder and forwards it to an io.Writer
func (w *Worker) TargzFolder(folder string, dest io.Writer) error {
	// Let's wire our writer together
	zipWriter := gzip.NewWriter(dest)
	defer zipWriter.Close()
	tarWriter := tar.NewWriter(zipWriter)
	defer tarWriter.Close()

	// Let's walk the target folder recursively and add each file entry to the tar archive
	return filepath.Walk(folder, func(path string, info os.FileInfo, walkerr error) error {
		if walkerr != nil {
			return fmt.Errorf("Error walking %s: %s", folder, walkerr)
		}

		if info.IsDir() {
			return nil
		}

		filename, err := filepath.Rel(folder, path)
		if err != nil {
			return fmt.Errorf("Error removing %s component from path %s: %s", folder, path, err)
		}

		log.Printf("Sending %s to archive", filename)

		file, err := os.Open(path)
		header := &tar.Header{
			Name:    filename,
			Size:    info.Size(),
			Mode:    0664,
			ModTime: info.ModTime(),
		}

		if err = tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("Error writing tar header for file %s: %s", path, err)
		}

		if _, err := io.Copy(tarWriter, file); err != nil {
			return fmt.Errorf("Error writing file %s to tar archive: %s", path, err)
		}

		return nil
	})
}

// UntargetTestingVolume copies data from /<host-data-volume>/<model>/data to
// /<host-data-volume>/<model>/train and removes targets from test files... using the problem
// workflow container.
func (w *Worker) UntargetTestingVolume(problemImage, testFolder, untargetedTestFolder string) (containerID string, err error) {
	return w.containerRuntime.RunImageInUntrustedContainer(
		problemImage,
		[]string{"-T", "detarget", "-i", "/hidden_data", "-s", "/submission_data"},
		map[string]string{
			testFolder:           "/hidden_data/test",
			untargetedTestFolder: "/submission_data/test",
		}, true)
}

// Train launches the submission container's train routines
func (w *Worker) Train(modelImage, trainFolder, testFolder, modelFolder string) (containerID string, err error) {
	return w.containerRuntime.RunImageInUntrustedContainer(
		modelImage,
		[]string{"-V", "/data", "-T", "train"},
		map[string]string{
			trainFolder: "/data/train",
			testFolder:  "/data/test",
			modelFolder: "/data/model",
		}, false)
}

// Predict launches the submission container's predict routines
func (w *Worker) Predict(modelImage, testFolder string) (containerID string, err error) {
	return w.containerRuntime.RunImageInUntrustedContainer(
		modelImage,
		[]string{"-V", "/data", "-T", "predict"},
		map[string]string{
			testFolder: "/data/test",
		}, true)
}

// ComputePerf analyses the prediction folders and computes a score for the model
func (w *Worker) ComputePerf(problemImage, trainFolder, testFolder, untargetedTestFolder, perfFolder string) (containerID string, err error) {
	return w.containerRuntime.RunImageInUntrustedContainer(
		problemImage,
		[]string{"-T", "perf", "-i", "/hidden_data", "-s", "/submission_data"},
		map[string]string{
			testFolder:           "/hidden_data/test",
			perfFolder:           "/hidden_data/perf",
			trainFolder:          "/submission_data/train",
			untargetedTestFolder: "/submission_data/test",
		}, true)
}
