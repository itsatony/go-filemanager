// processing.go
package filemanager

import (
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	PluginName string         `yaml:"plugin_name"`
	Params     map[string]any `yaml:"params"`
}

type OutputFormat struct {
	Format          string          `yaml:"format"`
	TargetFileNames []string        `yaml:"target_file_names"`
	StorageType     FileStorageType `yaml:"storage_type"` // public, private, temp
}

type Recipe struct {
	Name              string           `yaml:"name"`
	AcceptedMimeTypes []string         `yaml:"accepted_mime_types"`
	MinFileSize       int64            `yaml:"min_file_size"`
	MaxFileSize       int64            `yaml:"max_file_size"`
	ProcessingSteps   []ProcessingStep `yaml:"processing_steps"`
	OutputFormats     []OutputFormat   `yaml:"output_formats"`
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
		fm.LogTo("INFO", fmt.Sprintf("[FileManager.ProcessFile] Processing file(%s) Recipe(%s) not found.\n", file.FileName, recipeName))
		statusCh <- fileProcess
		return
	}
	fm.LogTo("DEBUG", fmt.Sprintf("[FileManager.ProcessFile] Processing file(%s) using recipe(%s)\n", file.FileName, recipeName))
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
		fm.LogTo("INFO", fmt.Sprintf("[FileManager.ProcessFile] Processing file(%s) MimeTypeCheck filed: \n%v\n", file.FileName, status))
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
		// fm.LogTo("DEBUG", fmt.Sprintf("[GO-FILEMANAGER.ProcessFile #3] Processing file ERROR: \n%v\n\n", status))
		fm.LogTo("INFO", fmt.Sprintf("[FileManager.ProcessFile] Processing file(%s) filesize check failed\n", file.FileName))
		statusCh <- fileProcess
		return
	}

	files := []*ManagedFile{file}

	for _, step := range recipe.ProcessingSteps {
		if step.PluginName == "" {
			continue
		}
		plugin, ok := fm.processingPlugins[step.PluginName]
		if !ok {
			status := ProcessingStatus{
				ProcessID:         fileProcess.ID,
				TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
				ProcessorName:     step.PluginName,
				StatusDescription: fmt.Sprintf("processing plugin(%s) not found", step.PluginName),
				Error:             fmt.Errorf("processing plugin(%s) not found", step.PluginName),
				Done:              true,
			}
			fileProcess.AddProcessingUpdate(status)
			// fm.LogTo("DEBUG", fmt.Sprintf("[GO-FILEMANAGER.ProcessFile #4] Processing file ERROR: \n%v\n\n", status))
			fm.LogTo("INFO", fmt.Sprintf("[FileManager.ProcessFile] Processing file(%s) Processing-Plugin(%s) not found!\n", file.FileName, step.PluginName))
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
			fm.LogTo("INFO", fmt.Sprintf("[FileManager.ProcessFile] Processing file(%s) Step failed:\n%v\n\n", file.FileName, status))
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
		// fm.LogTo("DEBUG", fmt.Sprintf("[GO-FILEMANAGER.ProcessFile #6] Processing file status update: \n%v\n\n", status))
		statusCh <- fileProcess
	}

	var outputFiles []*ManagedFile
	if file.MetaData == nil {
		file.MetaData = make(map[string]any)
	}
	file.MetaData["process_id"] = fileProcess.ID

	for _, outputFormat := range recipe.OutputFormats {
		for _, targetFilepathnameTemplate := range outputFormat.TargetFileNames {
			// Perform variable replacement in the target file name
			targetFilePath := ReplaceFileNameVariables(targetFilepathnameTemplate, file)
			// add file extension if not present
			if filepath.Ext(targetFilePath) == "" {
				targetFilePath = targetFilePath + filepath.Ext(file.FileName)
			}
			// fm.logger("DEBUG", fmt.Sprintf("################## [ProcessFile]: AFTER FILE-REPLACEMENT: targetFilePath(%s)\n", targetFilePath))
			fullFilePath, _, fileName := getFilePathAndName("", targetFilePath)
			// fm.logger("DEBUG", fmt.Sprintf("################## [ProcessFile]: AFTER EXTRACTION: fullFilePath(%s), fileName(%s)\n", fullFilePath, fileName))
			outputFile := &ManagedFile{
				FileName: fileName,
				MetaData: file.MetaData,
				FileSize: file.FileSize,
				MimeType: file.MimeType,
			}

			switch outputFormat.StorageType {
			case FileStorageTypePrivate:
				outputFile.LocalFilePath = fm.GetPrivateLocalFilePath(fullFilePath)
			case FileStorageTypeTemp:
				outputFile.LocalFilePath = fm.GetLocalTemporaryFilePath(fullFilePath)
			case FileStorageTypePublic:
				outputFile.LocalFilePath = fm.GetPublicLocalFilePath(fullFilePath)
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
				// fm.LogTo("DEBUG", fmt.Sprintf("[GO-FILEMANAGER.ProcessFile.OutputFormatCheck #6] Processing file ERROR: \n%v\n\n", status))
				statusCh <- fileProcess
				return
			}
			// fm.logger("DEBUG", fmt.Sprintf("################## [ProcessFile]: BASE-PATH-ADDITION: fullFilePath(%s)\n", outputFile.LocalFilePath))

			if outputFormat.StorageType == FileStorageTypePublic {
				outputFile.URL, _ = fm.GetPublicUrlForFile(outputFile.LocalFilePath)
			} else {
				outputFile.URL = ""
			}

			outputFile.Content = file.Content
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
				// fm.LogTo("DEBUG", fmt.Sprintf("[GO-FILEMANAGER.ProcessFile.FileSave #1] Processing file ERROR: \n%v\n\n", status))
				fm.LogTo("INFO", fmt.Sprintf("[FileManager.ProcessFile] Processing file(%s) Saving Result failed: \n%v\n", file.FileName, status))
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
	fm.LogTo("INFO", fmt.Sprintf("[FileManager.ProcessFile] Processing file(%s) COMPLETED: \n%v\n", file.FileName, status))
	statusCh <- fileProcess
}

func isValidMimeType(mimeType string, acceptedMimeTypes []string) bool {
	for _, accepted := range acceptedMimeTypes {
		// check lowercase matching and match as prefix
		if strings.HasPrefix(strings.ToLower(mimeType), strings.ToLower(accepted)) {
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

func ReplaceFileNameVariables(fileName string, file *ManagedFile) string {
	// Replace {metadata.whatever} with the corresponding value from file.MetaData
	metadataRegex := regexp.MustCompile(`{metadata\.([^}]+)}`)
	fileName = metadataRegex.ReplaceAllStringFunc(fileName, func(match string) string {
		key := strings.TrimPrefix(match, "{metadata.")
		key = strings.TrimSuffix(key, "}")
		value, ok := file.MetaData[key]
		if ok {
			return fmt.Sprintf("%v", value)
		}
		return ""
	})

	// Automatically add the correct file extension based on the MIME type
	extension := mime.TypeByExtension(file.FileName)
	if extension != "" {
		fileName = fileName + extension
	}

	return fileName
}
