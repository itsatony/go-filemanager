// upload.go
package filemanager

import (
	"fmt"
	"io"
	"os"
	"time"
)

func (fm *FileManager) HandleFileUpload(r io.Reader, fileProcess *FileProcess, statusCh chan<- *FileProcess) (*ManagedFile, error) {
	tempFile, err := os.CreateTemp(fm.localTempPath, "upload-*")
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()

	progressReader := &ProgressReader{
		Reader:      r,
		Size:        0,
		Uploaded:    0,
		StatusCh:    statusCh,
		FileProcess: fileProcess,
	}

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
	Reader      io.Reader
	Size        int64
	Uploaded    int64
	StatusCh    chan<- *FileProcess
	FileProcess *FileProcess
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
		status := ProcessingStatus{
			ProcessID:         r.FileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "FileUpload",
			StatusDescription: fmt.Sprintf("Uploading file: %s", r.FileProcess.IncomingFileName),
			Percentage:        percentage,
		}
		r.FileProcess.AddProcessingUpdate(status)
		select {
		case r.StatusCh <- r.FileProcess:
		default:
		}
	}

	return n, err
}
