// file_manager.go
package filemanager

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gabriel-vasile/mimetype"
	"gopkg.in/yaml.v2"
)

var ErrLocalFileNotFound = errors.New("local file not found")
var ErrUrlNotMapped = errors.New("url not mapped to local file")

type FileStorageType string

const (
	FileStorageTypePrivate FileStorageType = "private"
	FileStorageTypeTemp    FileStorageType = "temp"
	FileStorageTypePublic  FileStorageType = "public"
)

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

	files, err := ioutil.ReadDir(recipesDir)
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
