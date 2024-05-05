// processing.go
package filemanager

import (
	"errors"
	"fmt"
	"os"
	"time"
)

var (
	ErrRecipeNotFound           = errors.New("recipe not found")
	ErrInvalidMimeType          = errors.New("invalid MIME type")
	ErrInvalidFileSize          = errors.New("invalid file size")
	ErrProcessingPluginNotFound = errors.New("processing plugin not found")
)

type ProcessingPlugin interface {
	Process(files []*ManagedFile, fileProcess *FileProcess) ([]*ManagedFile, error)
}

type ProcessingStep struct {
	PluginName string         `yaml:"pluginName"`
	Params     map[string]any `yaml:"params"`
}

type OutputFormat struct {
	Format          string          `yaml:"format"`
	TargetFileNames []string        `yaml:"targetFileNames"`
	StorageType     FileStorageType `yaml:"storageType"` // public, private, temp
}

type Recipe struct {
	Name              string           `yaml:"name"`
	AcceptedMimeTypes []string         `yaml:"acceptedMimeTypes"`
	MinFileSize       int64            `yaml:"minFileSize"`
	MaxFileSize       int64            `yaml:"maxFileSize"`
	ProcessingSteps   []ProcessingStep `yaml:"processingSteps"`
	OutputFormats     []OutputFormat   `yaml:"outputFormats"`
}

type ProcessingResultFile struct {
	FileName      string
	LocalFilePath string
	URL           string
	FileSize      int64
	MimeType      string
}

type ProcessingStatus struct {
	ProcessID         string
	TimeStamp         int // js timestamp in unix milliseconds
	ProcessorName     string
	StatusDescription string
	Percentage        int
	Error             error
	Done              bool
	ResultingFiles    []ProcessingResultFile
}

func (fm *FileManager) ProcessFile(file *ManagedFile, recipeName string, fileProcess *FileProcess, statusCh chan<- *FileProcess) {
	defer close(statusCh)

	recipe, ok := fm.recipes[recipeName]
	if !ok {
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "RecipeCheck",
			StatusDescription: fmt.Sprintf("Recipe not found: %s", recipeName),
			Error:             fmt.Errorf("recipe not found: %s", recipeName),
			Done:              true,
		}
		fileProcess.AddProcessingUpdate(status)
		statusCh <- fileProcess
		return
	}

	if !isValidMimeType(file.MimeType, recipe.AcceptedMimeTypes) {
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "MimeTypeCheck",
			StatusDescription: fmt.Sprintf("Invalid MIME type: %s", file.MimeType),
			Error:             fmt.Errorf("invalid MIME type: %s", file.MimeType),
			Done:              true,
		}
		fileProcess.AddProcessingUpdate(status)
		statusCh <- fileProcess
		return
	}

	if file.FileSize < recipe.MinFileSize || file.FileSize > recipe.MaxFileSize {
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "FileSizeCheck",
			StatusDescription: fmt.Sprintf("Invalid file size: %d bytes", file.FileSize),
			Error:             fmt.Errorf("invalid file size: %d bytes", file.FileSize),
			Done:              true,
		}
		fileProcess.AddProcessingUpdate(status)
		statusCh <- fileProcess
		return
	}

	files := []*ManagedFile{file}

	for _, step := range recipe.ProcessingSteps {
		plugin, ok := fm.processingPlugins[step.PluginName]
		if !ok {
			status := ProcessingStatus{
				ProcessID:         fileProcess.ID,
				TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
				ProcessorName:     step.PluginName,
				StatusDescription: fmt.Sprintf("Processing plugin not found: %s", step.PluginName),
				Error:             fmt.Errorf("processing plugin not found: %s", step.PluginName),
				Done:              true,
			}
			fileProcess.AddProcessingUpdate(status)
			statusCh <- fileProcess
			return
		}

		processedFiles, err := plugin.Process(files, fileProcess)
		if err != nil {
			status := ProcessingStatus{
				ProcessID:         fileProcess.ID,
				TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
				ProcessorName:     step.PluginName,
				StatusDescription: fmt.Sprintf("Processing failed: %v", err),
				Error:             err,
				Done:              true,
			}
			fileProcess.AddProcessingUpdate(status)
			statusCh <- fileProcess
			return
		}

		files = processedFiles
		percentage := (len(files) * 100) / len(recipe.ProcessingSteps)
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     step.PluginName,
			StatusDescription: fmt.Sprintf("Processing step completed: %s", step.PluginName),
			Percentage:        percentage,
		}
		fileProcess.AddProcessingUpdate(status)
		statusCh <- fileProcess
	}

	var outputFiles []*ManagedFile

	for _, outputFormat := range recipe.OutputFormats {
		for _, targetFileName := range outputFormat.TargetFileNames {
			outputFile := &ManagedFile{
				FileName: targetFileName,
				MetaData: file.MetaData,
			}

			switch outputFormat.StorageType {
			case FileStorageTypePrivate:
				outputFile.LocalFilePath = fm.GetPrivateLocalFilePath(targetFileName)
			case FileStorageTypeTemp:
				outputFile.LocalFilePath = fm.GetLocalTemporaryFilePath(targetFileName)
			case FileStorageTypePublic:
				outputFile.LocalFilePath = fm.GetPublicLocalFilePath(targetFileName)
				outputFile.URL, _ = fm.GetPublicUrlForFile(outputFile.LocalFilePath)
			default:
				status := ProcessingStatus{
					ProcessID:         fileProcess.ID,
					TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
					ProcessorName:     "OutputFormatCheck",
					StatusDescription: fmt.Sprintf("Invalid storage type: %s", outputFormat.StorageType),
					Error:             fmt.Errorf("invalid storage type: %s", outputFormat.StorageType),
					Done:              true,
				}
				fileProcess.AddProcessingUpdate(status)
				statusCh <- fileProcess
				return
			}

			outputFile.Content = file.Content
			outputFile.MimeType = file.MimeType
			outputFile.FileSize = file.FileSize

			err := outputFile.Save()
			if err != nil {
				status := ProcessingStatus{
					ProcessID:         fileProcess.ID,
					TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
					ProcessorName:     "FileSave",
					StatusDescription: fmt.Sprintf("Failed to save output file: %v", err),
					Error:             err,
					Done:              true,
				}
				fileProcess.AddProcessingUpdate(status)
				statusCh <- fileProcess
				return
			}

			outputFiles = append(outputFiles, outputFile)
		}
	}

	var resultingFiles []ProcessingResultFile

	for _, outputFile := range outputFiles {
		resultingFile := ProcessingResultFile{
			FileName:      outputFile.FileName,
			LocalFilePath: outputFile.LocalFilePath,
			URL:           outputFile.URL,
			FileSize:      outputFile.FileSize,
			MimeType:      outputFile.MimeType,
		}
		resultingFiles = append(resultingFiles, resultingFile)
	}

	status := ProcessingStatus{
		ProcessID:         fileProcess.ID,
		TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
		ProcessorName:     "FileProcessing",
		StatusDescription: "File processing completed",
		Percentage:        100,
		Done:              true,
		ResultingFiles:    resultingFiles,
	}
	fileProcess.AddProcessingUpdate(status)
	fileProcess.LatestStatus.Done = true
	statusCh <- fileProcess
}

func isValidMimeType(mimeType string, acceptedMimeTypes []string) bool {
	for _, accepted := range acceptedMimeTypes {
		if mimeType == accepted {
			return true
		}
	}
	return false
}

// RunProcessingStep applies a single processing step to a ManagedFile.
func (fm *FileManager) RunProcessingStep(file *ManagedFile, pluginName string, params map[string]any, targetStorageType FileStorageType) (*ManagedFile, error) {
	fm.mu.RLock()
	plugin, exists := fm.processingPlugins[pluginName]
	fm.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("processing plugin not found: %s", pluginName)
	}

	// Wrap the file in a slice as some plugins may expect multiple files
	files := []*ManagedFile{file}

	// Create a dummy FileProcess to monitor the progress
	fileProcess := NewFileProcess(file.FileName, "SingleStepProcess")
	fileProcess.AddProcessingUpdate(ProcessingStatus{
		ProcessID:         fileProcess.ID,
		TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
		ProcessorName:     pluginName,
		StatusDescription: "Initiating single step processing",
	})

	// Execute the plugin processing
	processedFiles, err := plugin.Process(files, fileProcess)
	if err != nil {
		fileProcess.AddProcessingUpdate(ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     pluginName,
			StatusDescription: "Error during processing",
			Error:             err,
			Done:              true,
		})
		return nil, err
	}

	if len(processedFiles) == 0 {
		return nil, fmt.Errorf("no file processed by plugin: %s", pluginName)
	}

	// Assume the first file is the one we're interested in (since we provided one file)
	resultFile := processedFiles[0]

	// If a target storage type is specified, ensure the file is moved accordingly
	if targetStorageType != "" {
		localPath := fm.GetLocalPathForFile(targetStorageType, resultFile.FileName)
		if localPath != resultFile.LocalFilePath {
			err := os.Rename(resultFile.LocalFilePath, localPath)
			if err != nil {
				return nil, err
			}
			resultFile.LocalFilePath = localPath
		}
	}

	fileProcess.AddProcessingUpdate(ProcessingStatus{
		ProcessID:         fileProcess.ID,
		TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
		ProcessorName:     pluginName,
		StatusDescription: "Processing completed successfully",
		Done:              true,
	})

	return resultFile, nil
}
