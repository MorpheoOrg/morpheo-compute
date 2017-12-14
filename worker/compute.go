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
	// "io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/satori/go.uuid"

	"github.com/MorpheoOrg/morpheo-go-packages/client"
	"github.com/MorpheoOrg/morpheo-go-packages/common"
)

// Worker describes a worker (where it stores its data, which container runtime it uses...).
// Most importantly, it carefully implements all the steps of our learning/testing/prediction
// workflow.
//
// For an in-detail understanding of what these different steps do and how, check out Camille's
// awesome example: https://github.com/MorpheoOrg/hypnogram-wf
// The doc also gets there in detail: https://morpheoorg.github.io/morpheo/modules/learning.html
type Worker struct {
	ID uuid.UUID
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
	storage client.Storage
	peer    client.Peer
}

// Perfuplet describes the performance.json file, an output of learning tasks
type Perfuplet struct {
	Perf      float64            `json:"perf"`
	TrainPerf map[string]float64 `json:"train_perf"`
	TestPerf  map[string]float64 `json:"test_perf"`
}

// NewWorker creates a Worker instance
func NewWorker(dataFolder, trainFolder, testFolder, untargetedTestFolder, predFolder, perfFolder, modelFolder, problemImagePrefix, algoImagePrefix string, containerRuntime common.ContainerRuntime, storage client.Storage, peer client.Peer) *Worker {
	return &Worker{
		ID: uuid.NewV4(),

		dataFolder:           dataFolder,
		trainFolder:          trainFolder,
		testFolder:           testFolder,
		predFolder:           predFolder,
		perfFolder:           perfFolder,
		untargetedTestFolder: untargetedTestFolder,
		modelFolder:          modelFolder,

		problemImagePrefix: problemImagePrefix,
		algoImagePrefix:    algoImagePrefix,
		containerRuntime:   containerRuntime,

		storage: storage,
		peer:    peer,
	}
}

// HandleLearn manages a learning task (peer status updates, etc...)
func (w *Worker) HandleLearn(message []byte) (err error) {
	log.Println("[DEBUG][learn] Starting learning task")

	// Unmarshal the learn-uplet
	var task common.Learnuplet
	err = json.NewDecoder(bytes.NewReader(message)).Decode(&task)
	if err != nil {
		return fmt.Errorf("Error un-marshaling learn-uplet: %s -- Body: %s", err, message)
	}

	if err = task.Check(); err != nil {
		return fmt.Errorf("Error in train task: %s -- Body: %s", err, message)
	}

	// Update its status to pending on the peer
	_, _, err = w.peer.SetUpletWorker(task.Key, w.ID.String())
	if err != nil {
		return fmt.Errorf("Error setting uplet worker: %s", err)
	}

	err = w.LearnWorkflow(task)
	if err != nil {
		// TODO: handle fatal and non-fatal errors differently and set learnuplet status to failed only
		// if the error was fatal
		var m map[string]float64
		var f float64
		_, _, err2 := w.peer.ReportLearn(task.Key, common.TaskStatusFailed, f, m, m)
		if err2 != nil {
			return fmt.Errorf("Error in LearnWorkflow: %s. Error setting learnuplet status to failed on the peer: %s", err, err2)
		}
		return fmt.Errorf("Error in LearnWorkflow: %s", err)
	}
	return nil
}

// HandlePred manages a prediction task (peer status updates, etc...)
func (w *Worker) HandlePred(message []byte) (err error) {
	// log.Println("[DEBUG][pred] Starting predicting task")

	// // Unmarshal the learn-uplet
	// var task common.Preduplet
	// err = json.NewDecoder(bytes.NewReader(message)).Decode(&task)
	// if err != nil {
	// 	return fmt.Errorf("Error un-marshaling preduplet: %s -- Body: %s", err, message)
	// }

	// if err = task.Check(); err != nil {
	// 	return fmt.Errorf("Error in pred task: %s -- Body: %s", err, message)
	// }

	// // Update its status to pending on the peer
	// err = w.peer.UpdateUpletStatus(common.TypePredUplet, common.TaskStatusPending, task.Key, task.Worker)
	// if err != nil {
	// 	return fmt.Errorf("Error setting preduplet status to pending on the peer: %s", err)
	// }

	// err = w.PredWorkflow(task)
	// if err != nil {
	// 	// TODO: handle fatal and non-fatal errors differently and set preduplet status to failed only
	// 	// if the error was fatal
	// 	err2 := w.peer.UpdateUpletStatus(common.TypePredUplet, common.TaskStatusFailed, task.Key, task.Worker)
	// 	if err2 != nil {
	// 		return fmt.Errorf("2 Errors: Error in PredWorkflow: %s. Error setting preduplet status to failed on the peer: %s", err, err2)
	// 	}
	// 	return fmt.Errorf("Error in PredWorkflow: %s", err)
	// }
	return nil
}

// LearnWorkflow implements our learning workflow
func (w *Worker) LearnWorkflow(task common.Learnuplet) (err error) {
	log.Printf("[DEBUG][learn] Starting learning workflow for %s", task.Key)

	// Setup directory structure
	taskDataFolder := filepath.Join(w.dataFolder, task.Algo.String())
	trainFolder := filepath.Join(taskDataFolder, w.trainFolder)
	testFolder := filepath.Join(taskDataFolder, w.testFolder)
	untargetedTestFolder := filepath.Join(taskDataFolder, w.untargetedTestFolder)
	modelFolder := filepath.Join(taskDataFolder, w.modelFolder)
	perfFolder := filepath.Join(taskDataFolder, w.perfFolder)

	pathList := []string{taskDataFolder, trainFolder, testFolder, untargetedTestFolder, modelFolder, perfFolder}
	for _, path := range pathList {
		err = os.MkdirAll(path, os.ModeDir)
		if err != nil {
			return fmt.Errorf("Error creating folder under %s: %s", path, err)
		}
	}

	// Let's make sure these folders are wiped out once the task is done/failed
	defer os.RemoveAll(taskDataFolder)

	// Load problem workflow
	problemWorkflow, err := w.storage.GetProblemWorkflowBlob(task.Problem)
	if err != nil {
		return fmt.Errorf("Error pulling problem workflow %s from storage: %s", task.Problem, err)
	}
	problemImageName := fmt.Sprintf("%s-%s", w.problemImagePrefix, task.Problem)
	err = w.ImageLoad(problemImageName, problemWorkflow)
	if err != nil {
		return fmt.Errorf("Error loading problem workflow image %s in Docker daemon: %s", task.Problem, err)
	}
	problemWorkflow.Close()
	defer w.containerRuntime.ImageUnload(problemImageName)

	log.Println("[DEBUG][learn] 1st Image loaded")
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
		// Check that modelStart is set
		if uuid.Equal(uuid.Nil, task.ModelStart) {
			return fmt.Errorf("Error in learnuplet: ModelStart is a Nil uuid, although Rank is set to %d", task.Rank)
		}
		// Pull model from storage
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

	// Let's copy test data into untargetedTestFolder and remove targets
	_, err = w.UntargetTestingVolume(problemImageName, testFolder, untargetedTestFolder)
	if err != nil {
		return fmt.Errorf("Error preparing problem %s for model %s: %s", task.Problem, task.ModelStart, err)
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
		return fmt.Errorf("Error computing perf for problem %s and model (new) %s: %s", task.Problem, task.ModelEnd, err)
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

	if err := w.storage.PostModel(newModel, modelArchiveReader, modelArchiveStat.Size()); err != nil {
		return fmt.Errorf("Error streaming new model %s to storage: %s", task.ModelEnd, err)
	}
	modelArchiveReader.Close()

	// Let's send the perf file to the peer
	performanceFilePath := fmt.Sprintf("%s/performance.json", perfFolder)
	resultFile, err := os.Open(performanceFilePath)
	if err != nil {
		return fmt.Errorf("Error reading performance file %s: %s", performanceFilePath, err)
	}
	perfuplet := Perfuplet{}
	err = json.NewDecoder(resultFile).Decode(&perfuplet)
	if err != nil {
		return fmt.Errorf("Error un-marshaling performance file to JSON: %s", err)
	}
	if _, _, err := w.peer.ReportLearn(task.Key, common.TaskStatusDone, perfuplet.Perf, perfuplet.TrainPerf, perfuplet.TestPerf); err != nil {
		return fmt.Errorf("Error posting learn result %s to peer: %s", task.ModelEnd, err)
	}

	resultFile.Close()
	os.Remove(performanceFilePath)

	log.Printf("[INFO][learn] Train finished with success, cleaning up...")

	return
}

// // PredWorkflow handles our prediction tasks
// func (w *Worker) PredWorkflow(task common.Preduplet) (err error) {
// 	log.Println("[DEBUG][pred] Starting predicting workflow")

// 	// Setup directory structure
// 	taskDataFolder := filepath.Join(w.dataFolder, task.Model.String())
// 	testFolder := filepath.Join(taskDataFolder, w.testFolder)
// 	modelFolder := filepath.Join(taskDataFolder, w.modelFolder)
// 	predFolder := filepath.Join(testFolder, w.predFolder)

// 	err = os.MkdirAll(testFolder, os.ModeDir)
// 	if err != nil {
// 		return fmt.Errorf("Error creating test folder under %s: %s", testFolder, err)
// 	}
// 	err = os.MkdirAll(modelFolder, os.ModeDir)
// 	if err != nil {
// 		return fmt.Errorf("Error creating model folder under %s: %s", modelFolder, err)
// 	}
// 	err = os.MkdirAll(predFolder, os.ModeDir)
// 	if err != nil {
// 		return fmt.Errorf("Error creating pred folder under %s: %s", predFolder, err)
// 	}

// 	// Pulling data from storage to testFolder
// 	data, err := w.storage.GetDataBlob(task.Data)
// 	if err != nil {
// 		return fmt.Errorf("Error pulling data %s from storage: %s", task.Data, err)
// 	}
// 	path := fmt.Sprintf("%s/%s", testFolder, task.Data)
// 	dataFile, err := os.Create(path)
// 	if err != nil {
// 		return fmt.Errorf("Error creating file %s: %s", path, err)
// 	}
// 	n, err := io.Copy(dataFile, data)
// 	if err != nil {
// 		return fmt.Errorf("Error copying data file %s (%d bytes written): %s", path, n, err)
// 	}
// 	dataFile.Close()
// 	data.Close()

// 	// Pull model from storage and store it in modelFolder
// 	model, err := w.storage.GetModelBlob(task.Model)
// 	if err != nil {
// 		return fmt.Errorf("Error pulling model %s from storage: %s", task.Model, err)
// 	}

// 	err = w.UntargzInFolder(modelFolder, model)
// 	if err != nil {
// 		return fmt.Errorf("Error un-tar-gz-ing model: %s", err)
// 	}
// 	model.Close()

// 	// Rename model
// 	files, err := ioutil.ReadDir(modelFolder)
// 	if err != nil {
// 		return fmt.Errorf("Error reading modelFolder: %s", err)
// 	}
// 	if len(files) != 1 {
// 		return fmt.Errorf("Error: several files in modelFolder")
// 	}
// 	for _, f := range files {
// 		oldpath := filepath.Join(modelFolder, f.Name())
// 		newpath := filepath.Join(modelFolder, "model_trained.json")
// 		if err = os.Rename(oldpath, newpath); err != nil {
// 			return fmt.Errorf("Error renaming model: %s", err)
// 		}
// 	}

// 	// Pull associated algo and load it into a container
// 	modelInfo, err := w.storage.GetModel(task.Model)
// 	if err != nil {
// 		return fmt.Errorf("Error retrieving model %s metadata: %s", task.Model, err)
// 	}
// 	algo, err := w.storage.GetAlgoBlob(modelInfo.Algo)
// 	if err != nil {
// 		return fmt.Errorf("Error pulling algo %s from storage: %s", modelInfo.Algo, err)
// 	}
// 	algoImageName := fmt.Sprintf("%s-%s", w.algoImagePrefix, modelInfo.Algo)
// 	err = w.ImageLoad(algoImageName, algo)
// 	if err != nil {
// 		return fmt.Errorf("Error loading algo image %s in Docker daemon: %s", algoImageName, err)
// 	}
// 	algo.Close()
// 	defer w.containerRuntime.ImageUnload(algoImageName)

// 	// Let's pass the prediction task to our execution backend, now that everything should be in place
// 	_, err = w.Predict(algoImageName, testFolder, predFolder, modelFolder)
// 	if err != nil {
// 		return fmt.Errorf("Error in pred task: %s -- Body: %s", err, task)
// 	}

// 	// Let's send the prediction to Storage and address & status to Peer
// 	// Check if prediction file exists
// 	path = filepath.Join(predFolder, task.Data.String())
// 	if _, err := os.Stat(path); os.IsNotExist(err) {
// 		return fmt.Errorf("Error: missing prediction file for data %s", task.Data.String())
// 	}

// 	// Open file and retrieve size
// 	file, err := os.Open(path)
// 	if err != nil {
// 		return fmt.Errorf("Error opening prediction file from path %s: %s", path, err)
// 	}
// 	defer file.Close()

// 	stat, err := file.Stat()
// 	if err != nil {
// 		return fmt.Errorf("Error retrieving file stat: %s", err)
// 	}
// 	filesize := stat.Size()

// 	// // Let's tar-gz the file
// 	// var targzFile bytes.Buffer
// 	// err = TargzFile(file, &targzFile)
// 	// if err != nil {
// 	// 	log.Println("Error compressing prediction file from path %s: %s", path, err)
// 	// 	continue
// 	// }

// 	// Send the prediction to storage
// 	log.Println("[DEBUG][pred] sending the predictions to Storage...")
// 	newPrediction := common.NewPrediction()
// 	err = w.storage.PostPrediction(newPrediction, file, filesize)
// 	if err != nil {
// 		return fmt.Errorf("Error streaming new prediction %s to storage: %s", newPrediction.ID, err)
// 	}

// 	// Send status and prediction address to Peer
// 	log.Println("[DEBUG][pred] sending the status and prediction UUID to Peer...")
// 	err = w.peer.PostPredResult(task.Key, common.TaskStatusDone, newPrediction.ID)
// 	if err != nil {
// 		return fmt.Errorf("Error setting preduplet status to 'done' on the peer: %s", err)
// 	}

// 	log.Printf("[INFO][pred] Prediction finished with success, cleaning up...")
// 	return nil
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

	log.Printf("[DEBUG][containerRuntime] Loading image %s...", imageName)
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
			return fmt.Errorf("Error reading model tar archive: %s", err)
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

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("Error opening file from path %s: %s", path, err)
		}
		defer file.Close()
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

// TargzFile tars and gzips a file and forwards it to an io.Writer
func TargzFile(file *os.File, dest io.Writer) error {
	// Let's wire our writer together
	zipWriter := gzip.NewWriter(dest)
	defer zipWriter.Close()
	tarWriter := tar.NewWriter(zipWriter)
	defer tarWriter.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Error getting file info: %s", err)
	}
	// Let's create the header
	header := &tar.Header{
		Name:    stat.Name(),
		Size:    stat.Size(),
		Mode:    0664,
		ModTime: stat.ModTime(),
	}
	// write the header to the tarball archive
	if err = tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("Error writing tar header for file %s", err)
	}
	if _, err := io.Copy(tarWriter, file); err != nil {
		return fmt.Errorf("Error writing file %s to tar archive", err)
	}
	return nil
}

// SetupDirectories creates all the required directory. Useful for testing
func (w *Worker) SetupDirectories(taskDataFolder string, filemode os.FileMode) error {
	trainFolder := filepath.Join(taskDataFolder, w.trainFolder)
	testFolder := filepath.Join(taskDataFolder, w.testFolder)
	modelFolder := filepath.Join(taskDataFolder, w.modelFolder)
	predFolder := filepath.Join(testFolder, w.predFolder)
	untargetedTestFolder := filepath.Join(taskDataFolder, w.untargetedTestFolder)
	perfFolder := filepath.Join(taskDataFolder, w.perfFolder)

	pathList := []string{taskDataFolder, trainFolder, testFolder, modelFolder, predFolder, untargetedTestFolder, perfFolder}
	for _, path := range pathList {
		err := os.MkdirAll(path, filemode)
		if err != nil {
			return fmt.Errorf("Error creating folder under %s: %s", path, err)
		}
	}
	return nil
}

// UntargetTestingVolume copies test data from /<host-data-volume>/<model>/test to
// /<host-data-volume>/<model>/untargeted_test and removes targets from files... using the problem
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
func (w *Worker) Predict(modelImage, testFolder string, predFolder string, modelFolder string) (containerID string, err error) {
	return w.containerRuntime.RunImageInUntrustedContainer(
		modelImage,
		[]string{"-V", "/data", "-T", "predict"},
		map[string]string{
			testFolder:  "/data/test",
			predFolder:  "/data/test/pred",
			modelFolder: "/data/model",
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
