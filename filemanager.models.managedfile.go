package filemanager

import (
	"mime"
	"os"
	"path/filepath"
)

type ManagedFile struct {
	FileName         string         `json:"fileName"`
	MimeType         string         `json:"mimetype"`
	URL              string         `json:"url"`
	LocalFilePath    string         `json:"localFilePath"`
	FileSize         int64          `json:"fileSize"`
	MetaData         map[string]any `json:"metaData"`
	ProcessingErrors []string       `json:"processingErrors"`
	Content          []byte         `json:"-"`
}

func (entity *ManagedFile) GetFileName() string {
	return entity.FileName
}

func (entity *ManagedFile) GetLocalFilePathWithoutFileName() string {
	filepath := filepath.Dir(entity.LocalFilePath)
	return filepath
}

func (entity *ManagedFile) UpdateMimeType() string {
	if entity.LocalFilePath != "" {
		contentType, err := GuessMimeType(entity.LocalFilePath)
		if err != nil {
			return ""
		}
		entity.MimeType = contentType
	}
	return entity.MimeType
}

func (entity *ManagedFile) UpdateFilesize() int64 {
	if entity.FileSize == 0 && entity.LocalFilePath != "" {
		fileInfo, err := os.Stat(entity.LocalFilePath)
		if err != nil {
			return 0
		}
		entity.FileSize = fileInfo.Size()
	}
	return entity.FileSize
}

func (entity *ManagedFile) EnsureFileIsLocal(fm *FileManager, target FileStorageType) (file *ManagedFile, err error) {
	if entity.LocalFilePath == "" || (entity.LocalFilePath != "" && !FileExists(entity.LocalFilePath)) {

		// decide where to download the file to based on the target var and get the respective local path from the FileManager
		localFilePath := fm.GetLocalPathForFile(target, entity.FileName)
		err = DownloadFileFromUrl(entity.URL, localFilePath)
		if err != nil {
			return file, err
		}
		entity.LocalFilePath = localFilePath
		if target == FileStorageTypePublic && entity.URL == "" {
			entity.URL, err = fm.GetPublicUrlForFile(entity.LocalFilePath)
			if err != nil {
				return entity, err
			}
		}
	}
	return entity, nil
}

func (entity *ManagedFile) EnsurePublicURL(fm *FileManager) (pubUrl string, err error) {
	pubUrl = ""
	if entity.URL != "" {
		pubUrl = entity.URL
		return pubUrl, nil
	}
	_, err = entity.EnsureFileIsLocal(fm, FileStorageTypePublic)
	return pubUrl, err
}

func (entity *ManagedFile) SetMetaData(key string, value any) {
	entity.MetaData[key] = value
}

func (entity *ManagedFile) GetMetaData(key string) (value any) {
	val, ok := entity.MetaData[key]
	if ok {
		return val
	}
	return nil
}

func (file *ManagedFile) Save() error {
	// Create the directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(file.LocalFilePath), os.ModePerm)
	if err != nil {
		return err
	}

	// Open the file for writing
	outputFile, err := os.Create(file.LocalFilePath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Write the file content to the output file
	_, err = outputFile.Write(file.Content)
	if err != nil {
		return err
	}

	// Update the file metadata
	file.FileSize = int64(len(file.Content))
	file.MimeType = mime.TypeByExtension(filepath.Ext(file.LocalFilePath))

	return nil
}
