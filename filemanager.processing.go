// processing.go
package filemanager

import (
	"errors"
	"fmt"
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
	Format         string          `yaml:"format"`
	TargetFileName string          `yaml:"targetFileName"`
	StorageType    FileStorageType `yaml:"storageType"` // public, private, temp
}

type Recipe struct {
	Name              string           `yaml:"name"`
	AcceptedMimeTypes []string         `yaml:"acceptedMimeTypes"`
	MinFileSize       int64            `yaml:"minFileSize"`
	MaxFileSize       int64            `yaml:"maxFileSize"`
	ProcessingSteps   []ProcessingStep `yaml:"processingSteps"`
	OutputFormats     []OutputFormat   `yaml:"outputFormats"`
}

type ProcessingStatus struct {
	ProcessID         string
	TimeStamp         int // js timestamp in unix milliseconds
	ProcessorName     string
	StatusDescription string
	Percentage        int
	Error             error
	Done              bool
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
			Error:             ErrRecipeNotFound,
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
			Error:             ErrInvalidMimeType,
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
			Error:             ErrInvalidFileSize,
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
				Error:             ErrProcessingPluginNotFound,
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

	for _, outputFormat := range recipe.OutputFormats {
		for _, file := range files {
			switch outputFormat.StorageType {
			case FileStorageTypePrivate:
				file.LocalFilePath = fm.GetPrivateLocalFilePath(outputFormat.TargetFileName)
			case FileStorageTypeTemp:
				file.LocalFilePath = fm.GetLocalTemporaryFilePath(outputFormat.TargetFileName)
			case FileStorageTypePublic:
				file.LocalFilePath = fm.GetPublicLocalFilePath(outputFormat.TargetFileName)
				file.URL, _ = fm.GetPublicUrlForFile(file.LocalFilePath)
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

			err := file.Save()
			if err != nil {
				status := ProcessingStatus{
					ProcessID:         fileProcess.ID,
					TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
					ProcessorName:     "FileSave",
					StatusDescription: fmt.Sprintf("Failed to save file: %v", err),
					Error:             err,
					Done:              true,
				}
				fileProcess.AddProcessingUpdate(status)
				statusCh <- fileProcess
				return
			}
		}
	}

	status := ProcessingStatus{
		ProcessID:         fileProcess.ID,
		TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
		ProcessorName:     "FileProcessing",
		StatusDescription: "File processing completed",
		Percentage:        100,
		Done:              true,
	}
	fileProcess.AddProcessingUpdate(status)
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
