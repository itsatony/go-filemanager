package filemanager

import (
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

var (
	ErrNilResponseBody = errors.New("response body is nil")
)

const idAlphabet string = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_"

func NID(prefix string, length int) (nid string) {
	nid, err := gonanoid.Generate(idAlphabet, length)
	if err != nil {
		nid = strconv.FormatInt(time.Now().UnixMicro(), 10)
	}
	if len(prefix) > 0 {
		nid = prefix + "_" + nid
	}
	return nid
}

// CreateManagedFileFromPath creates a ManagedFile from a given local path.
func (fm *FileManager) CreateManagedFileFromPath(localPath string, targetStorageType FileStorageType) (*ManagedFile, error) {
	if !FileExists(localPath) {
		return nil, ErrLocalFileNotFound
	}

	fileSize := int64(0)
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return nil, err
	}
	fileSize = fileInfo.Size()

	mimeType, err := GuessMimeType(localPath)
	if err != nil {
		return nil, err
	}

	managedFile := &ManagedFile{
		FileName:      filepath.Base(localPath),
		LocalFilePath: localPath,
		FileSize:      fileSize,
		MimeType:      mimeType,
		MetaData:      make(map[string]any),
	}

	// Move file if not in the correct location
	targetPath := fm.GetLocalPathForFile(targetStorageType, managedFile.FileName)
	if localPath != targetPath {
		err = os.Rename(localPath, targetPath)
		if err != nil {
			return nil, err
		}
		managedFile.LocalFilePath = targetPath
	}

	if targetStorageType == FileStorageTypePublic {
		pubUrl, err := fm.GetPublicUrlForFile(managedFile.LocalFilePath)
		if err != nil {
			return nil, err
		}
		managedFile.URL = pubUrl
	}

	return managedFile, nil
}

// CreateManagedFileFromFileHeader creates a ManagedFile from a multipart.FileHeader which is typical in HTTP file uploads.
func (fm *FileManager) CreateManagedFileFromFileHeader(fileHeader *multipart.FileHeader, targetStorageType FileStorageType) (*ManagedFile, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	localFilePath := fm.GetLocalPathForFile(targetStorageType, fileHeader.Filename)
	outFile, err := os.Create(localFilePath)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		return nil, err
	}

	fileSize := int64(fileHeader.Size)
	mimeType, err := GuessMimeType(localFilePath)
	if err != nil {
		return nil, err
	}

	return &ManagedFile{
		FileName:      filepath.Base(fileHeader.Filename),
		LocalFilePath: localFilePath,
		FileSize:      fileSize,
		MimeType:      mimeType,
		MetaData:      make(map[string]any),
	}, nil
}

// CreateManagedFileFromResponseBody creates a ManagedFile from a response body. will NOT CLOSE the response body.
func (fm *FileManager) CreateManagedFileFromResponseBody(filename string, responseBody io.ReadCloser, targetStorageType FileStorageType) (*ManagedFile, error) {
	if responseBody == nil {
		return nil, ErrNilResponseBody
	}

	localFilePath := fm.GetLocalPathForFile(targetStorageType, filename)
	outFile, err := os.Create(localFilePath)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()

	writtenBytes, err := io.Copy(outFile, responseBody)
	if err != nil {
		return nil, err
	}

	mimeType, err := GuessMimeType(localFilePath)
	if err != nil {
		return nil, err
	}

	return &ManagedFile{
		FileName:      filepath.Base(filename),
		LocalFilePath: localFilePath,
		FileSize:      writtenBytes,
		MimeType:      mimeType,
		MetaData:      make(map[string]any),
	}, nil
}

func (fm *FileManager) LogTo(level string, message string) {
	if fm.logger != nil {
		fm.logger(level, message)
	}
}

func getFilePathAndName(localBasePath string, filePathName string) (fullPath string, dirPath string, pureFileName string) {
	// Join the local base path and the file name to form the full path
	fullPath = filepath.Join(localBasePath, filePathName)

	// Extract the directory path without the filename
	dirPath = filepath.Dir(fullPath)

	// Extract the pure filename (including extension)
	pureFileName = filepath.Base(fullPath)

	return fullPath, dirPath, pureFileName
}
