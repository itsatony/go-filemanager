// file_manager.go

// Package filemanager provides a flexible and extensible file management system
// for handling file storage, retrieval, and processing using a plugin-based architecture.
//
// The main components of the filemanager package are:
//
// - FileManager: The central structure that manages file storage, retrieval, and processing.
//   It provides methods for initializing the FileManager, adding processing plugins,
//   loading recipes, and processing files.
//
// - ManagedFile: Represents a file managed by the FileManager. It contains information
//   such as the file name, MIME type, URL, local file path, file size, metadata, and
//   processing errors.
//
// - FileProcess: Represents a file processing task. It includes the incoming file name,
//   recipe name, and processing updates. It provides methods for adding processing updates
//   and retrieving the latest processing status.
//
// - ProcessingPlugin: An interface that defines the contract for processing plugins.
//   Processing plugins are responsible for processing files based on specific requirements.
//
// - Recipe: Represents a processing recipe that specifies the accepted MIME types, file size
//   constraints, processing steps, and output formats for a file processing task.
//
// - ProcessingStatus: Represents the status of a file processing task. It includes information
//   such as the process ID, timestamp, processor name, status description, progress percentage,
//   error (if any), completion status, and resulting files.
//
// - ProcessingResultFile: Represents a resulting file from a file processing task. It contains
//   information such as the file name, local file path, URL, file size, and MIME type.
//
// The filemanager package provides a high-level API for managing files and processing them
// using various plugins and recipes. It abstracts the complexities of file storage and
// processing, allowing developers to focus on defining processing plugins and recipes to
// suit their specific requirements.

package filemanager

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gabriel-vasile/mimetype"
	"gopkg.in/yaml.v2"
)

var (
	ErrLocalFileNotFound = errors.New("local file not found")
	ErrUrlNotMapped      = errors.New("url not mapped to local file")
)

const FILE_PROCESS_ID_LENGTH = 16
const FILE_PROCESS_ID_PREFIX = "FP"

type FileStorageType string

const (
	FileStorageTypePrivate FileStorageType = "private"
	FileStorageTypeTemp    FileStorageType = "temp"
	FileStorageTypePublic  FileStorageType = "public"
)

type FileProcess struct {
	ID                string
	IncomingFileName  string
	RecipeName        string
	ProcessingUpdates []ProcessingStatus
	LatestStatus      *ProcessingStatus
}

func (fp *FileProcess) AddProcessingUpdate(update ProcessingStatus) {
	fp.ProcessingUpdates = append(fp.ProcessingUpdates, update)
	fp.LatestStatus = &update
}

func (fp *FileProcess) GetLatestProcessingStatus() *ProcessingStatus {
	return fp.LatestStatus
}

func NewFileProcess(incomingFileName, recipeName string) *FileProcess {
	id := NID(FILE_PROCESS_ID_PREFIX, FILE_PROCESS_ID_LENGTH)
	return &FileProcess{
		ID:               id,
		IncomingFileName: incomingFileName,
		RecipeName:       recipeName,
	}
}

type FileManager struct {
	publicLocalBasePath  string
	privateLocalBasePath string
	baseUrl              string
	localTempPath        string
	processingPlugins    map[string]ProcessingPlugin
	recipes              map[string]Recipe
	mu                   sync.RWMutex
}

func NewFileManager(publicLocalBasePath, privateLocalBasePath, baseUrl, tempPath string) *FileManager {
	return &FileManager{
		publicLocalBasePath:  publicLocalBasePath,
		privateLocalBasePath: privateLocalBasePath,
		baseUrl:              baseUrl,
		localTempPath:        tempPath,
		processingPlugins:    make(map[string]ProcessingPlugin),
		recipes:              make(map[string]Recipe),
	}
}

func (fm *FileManager) AddProcessingPlugin(name string, plugin ProcessingPlugin) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.processingPlugins[name] = plugin
}

func (fm *FileManager) LoadRecipes(recipesDir string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	files, err := os.ReadDir(recipesDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if filepath.Ext(file.Name()) != ".yaml" {
			continue
		}

		filePath := filepath.Join(recipesDir, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var recipe Recipe
		err = yaml.Unmarshal(data, &recipe)
		if err != nil {
			continue
		}

		fm.recipes[recipe.Name] = recipe
	}

	return nil
}

func (fm *FileManager) GetRecipe(name string) (Recipe, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	if _, ok := fm.recipes[name]; !ok {
		return Recipe{}, ErrRecipeNotFound
	}
	return fm.recipes[name], nil
}

func (aifm *FileManager) GetLocalPathForFile(target FileStorageType, filename string) string {
	var localPath string
	switch target {
	case FileStorageTypePrivate:
		localPath = aifm.GetPrivateLocalFilePath(filename)
	case FileStorageTypeTemp:
		localPath = aifm.GetLocalTemporaryFilePath(filename)
	case FileStorageTypePublic:
		localPath = aifm.GetPublicLocalFilePath(filename)
	}
	return localPath
}

func (aifm *FileManager) GetPublicUrlForFile(localFilePath string) (url string, err error) {
	// first check if the local file path has our local public base path - if not, return error
	if !strings.HasPrefix(localFilePath, aifm.publicLocalBasePath) {
		return url, ErrLocalFileNotFound
	}
	relativePath := strings.TrimPrefix(localFilePath, aifm.publicLocalBasePath)
	return path.Join(aifm.baseUrl, relativePath), nil
}

func (aifm *FileManager) GetPublicLocalBasePath() string {
	return aifm.publicLocalBasePath
}

func (aifm *FileManager) GetPrivateLocalBasePath() string {
	return aifm.privateLocalBasePath
}

func (aifm *FileManager) GetBaseUrl() string {
	return aifm.baseUrl
}

func (aifm *FileManager) GetLocalPathOfUrl(url string) (localPath string, err error) {
	// first check if the url has our url prefix - if not, return error
	if !strings.HasPrefix(url, aifm.baseUrl) {
		return localPath, ErrUrlNotMapped
	}
	// get the relative path and filename from the url and append it to the local base path
	relativePath := strings.TrimPrefix(url, aifm.baseUrl)
	localPath = path.Join(aifm.publicLocalBasePath, relativePath)
	// check if the file exists
	if !FileExists(localPath) {
		return localPath, ErrLocalFileNotFound
	}
	return localPath, nil
}

func (aifm *FileManager) GetPublicLocalFilePath(fileName string) string {
	return path.Join(aifm.publicLocalBasePath, fileName)
}

func (aifm *FileManager) GetPrivateLocalFilePath(fileName string) string {
	return path.Join(aifm.privateLocalBasePath, fileName)
}

func (aifm *FileManager) GetLocalTemporaryPath() string {
	return aifm.localTempPath
}

func (aifm *FileManager) GetLocalTemporaryFilePath(fileName string) string {
	return path.Join(aifm.localTempPath, fileName)
}

func GuessMimeType(filepath string) (string, error) {
	mtype, err := mimetype.DetectFile(filepath)
	if err != nil {
		return "", err
	}
	mime := mtype.String()
	return mime, err
}

func DownloadFileFromUrl(url string, localFilePath string) (err error) {
	// Download the file from url
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	file, err := os.Create(localFilePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}
	return nil
}

func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}
