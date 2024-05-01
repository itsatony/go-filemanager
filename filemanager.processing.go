// processing.go
package filemanager

import (
	"errors"
	"fmt"
)

var (
	ErrRecipeNotFound           = errors.New("recipe not found")
	ErrInvalidMimeType          = errors.New("invalid MIME type")
	ErrInvalidFileSize          = errors.New("invalid file size")
	ErrProcessingPluginNotFound = errors.New("processing plugin not found")
)

type ProcessingPlugin interface {
	Process(files []*ManagedFile) ([]*ManagedFile, error)
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
	Percentage int
	Error      error
	Done       bool
}

func (fm *FileManager) ProcessFile(file *ManagedFile, recipeName string, statusCh chan<- ProcessingStatus) {
	defer close(statusCh)

	recipe, ok := fm.recipes[recipeName]
	if !ok {
		statusCh <- ProcessingStatus{Error: ErrRecipeNotFound, Done: true}
		return
	}

	// Validate the file against the recipe's accepted MIME types and file size constraints
	if !isValidMimeType(file.MimeType, recipe.AcceptedMimeTypes) {
		statusCh <- ProcessingStatus{Error: ErrInvalidMimeType, Done: true}
		return
	}

	if file.FileSize < recipe.MinFileSize || file.FileSize > recipe.MaxFileSize {
		statusCh <- ProcessingStatus{Error: ErrInvalidFileSize, Done: true}
		return
	}

	files := []*ManagedFile{file}

	for _, step := range recipe.ProcessingSteps {
		plugin, ok := fm.processingPlugins[step.PluginName]
		if !ok {
			statusCh <- ProcessingStatus{Error: ErrProcessingPluginNotFound, Done: true}
			return
		}

		processedFiles, err := plugin.Process(files)
		if err != nil {
			statusCh <- ProcessingStatus{Error: err, Done: true}
			return
		}

		files = processedFiles
		statusCh <- ProcessingStatus{Percentage: (len(files) * 100) / len(recipe.ProcessingSteps)}
	}

	// Store the processed files based on the output formats and target file names defined in the recipe
	for _, outputFormat := range recipe.OutputFormats {
		for _, file := range files {
			// Set the storage type and update the file URL based on the storage type
			switch outputFormat.StorageType {
			case FileStorageTypePrivate:
				file.LocalFilePath = fm.GetPrivateLocalFilePath(outputFormat.TargetFileName)
			case FileStorageTypeTemp:
				file.LocalFilePath = fm.GetLocalTemporaryFilePath(outputFormat.TargetFileName)
			case FileStorageTypePublic:
				file.LocalFilePath = fm.GetPublicLocalFilePath(outputFormat.TargetFileName)
				file.URL, _ = fm.GetPublicUrlForFile(file.LocalFilePath)
			default:
				statusCh <- ProcessingStatus{Error: fmt.Errorf("invalid storage type: %s", outputFormat.StorageType), Done: true}
				return
			}

			// Save the processed file
			err := file.Save()
			if err != nil {
				statusCh <- ProcessingStatus{Error: err, Done: true}
				return
			}
		}
	}

	statusCh <- ProcessingStatus{Percentage: 100, Done: true}
}

func isValidMimeType(mimeType string, acceptedMimeTypes []string) bool {
	for _, accepted := range acceptedMimeTypes {
		if mimeType == accepted {
			return true
		}
	}
	return false
}
