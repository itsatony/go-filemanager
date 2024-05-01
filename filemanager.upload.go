// upload.go
package filemanager

import (
	"io"
	"os"
)

func (fm *FileManager) HandleFileUpload(r io.Reader, statusCh chan<- ProcessingStatus) (*ManagedFile, error) {
	tempFile, err := os.CreateTemp(fm.localTempPath, "upload-*")
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()

	// Create a new progress reader to track upload progress
	progressReader := &ProgressReader{
		Reader:   r,
		Size:     0,
		Uploaded: 0,
		StatusCh: statusCh,
	}

	// Copy the file content to the temporary file using the progress reader
	_, err = io.Copy(tempFile, progressReader)
	if err != nil {
		return nil, err
	}

	managedFile := &ManagedFile{
		FileName:      tempFile.Name(),
		LocalFilePath: tempFile.Name(),
	}

	managedFile.UpdateMimeType()
	managedFile.UpdateFilesize()

	return managedFile, nil
}

type ProgressReader struct {
	Reader   io.Reader
	Size     int64
	Uploaded int64
	StatusCh chan<- ProcessingStatus
}

func (r *ProgressReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.Uploaded += int64(n)

	if r.Size == 0 {
		if file, ok := r.Reader.(*os.File); ok {
			fileInfo, err := file.Stat()
			if err == nil {
				r.Size = fileInfo.Size()
			}
		}
	}

	if r.Size > 0 {
		percentage := int(float64(r.Uploaded) / float64(r.Size) * 100)
		select {
		case r.StatusCh <- ProcessingStatus{Percentage: percentage}:
		default:
			// Non-blocking send to avoid blocking the upload progress
		}
	}

	return n, err
}
