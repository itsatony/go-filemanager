// upload.go
package filemanager

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func (fm *FileManager) HandleFileUpload(r io.Reader, fileProcess *FileProcess, statusCh chan<- *FileProcess) (*ManagedFile, error) {
	// todo: make incoming filename safe!
	tempFile, err := os.CreateTemp(fm.localTempPath, "upload-*_."+filepath.Ext(fileProcess.IncomingFileName))
	if err != nil {
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "FileUpload",
			StatusDescription: "Failed to create temporary file",
			Error:             err,
			Done:              true,
		}
		fileProcess.AddProcessingUpdate(status)
		statusCh <- fileProcess
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
		status := ProcessingStatus{
			ProcessID:         fileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "FileUpload",
			StatusDescription: "Failed to save uploaded file",
			Error:             err,
			Done:              true,
		}
		fileProcess.AddProcessingUpdate(status)

		fm.LogTo("DEBUG", fmt.Sprintf("[GO-FILEMANAGER #1] Uploading file ERROR: %s - %d%% \n%v", fileProcess.IncomingFileName, 100, status))
		statusCh <- fileProcess
		return nil, err
	}

	fpath, _, fname := getFilePathAndName("", tempFile.Name())

	managedFile := &ManagedFile{
		FileName:      fname,
		LocalFilePath: fpath,
	}

	managedFile.UpdateMimeType()
	managedFile.UpdateFilesize()

	resultingFile := ProcessingResultFile{
		FileName:      managedFile.FileName,
		LocalFilePath: managedFile.LocalFilePath,
		FileSize:      managedFile.FileSize,
		MimeType:      managedFile.MimeType,
	}

	status := ProcessingStatus{
		ProcessID:         fileProcess.ID,
		TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
		ProcessorName:     "FileUpload",
		StatusDescription: "File uploaded successfully",
		Done:              false,
		ResultingFiles:    []ProcessingResultFile{resultingFile},
	}
	if progressReader.FileProcess != nil && progressReader.FileProcess.LatestStatus != nil {
		status.Percentage = progressReader.FileProcess.LatestStatus.Percentage
		if status.Percentage == 100 {
			status.Done = true
		}
	}
	fileProcess.AddProcessingUpdate(status)
	fm.LogTo("DEBUG", fmt.Sprintf("[GO-FILEMANAGER #2] Uploading file: %s - %d%% \n%v", fileProcess.IncomingFileName, 100, status))
	statusCh <- fileProcess

	return managedFile, nil
}

type ProgressReader struct {
	Reader      io.Reader
	Size        int64
	Uploaded    int64
	StatusCh    chan<- *FileProcess
	FileProcess *FileProcess
	Done        bool
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

	if r.Size > 0 && !r.Done {
		percentage := int(float64(r.Uploaded) / float64(r.Size) * 100)
		if percentage > 100 {
			percentage = 100
		}
		status := ProcessingStatus{
			ProcessID:         r.FileProcess.ID,
			TimeStamp:         int(time.Now().UnixNano() / int64(time.Millisecond)),
			ProcessorName:     "FileUpload",
			StatusDescription: fmt.Sprintf("Uploading file: %s", r.FileProcess.IncomingFileName),
			Percentage:        percentage,
		}
		if percentage == 100 {
			status.Done = true
		} else {
			r.FileProcess.AddProcessingUpdate(status)
			r.StatusCh <- r.FileProcess
		}
		// select {
		// case r.StatusCh <- r.FileProcess:
		// default:
		// }
	}

	return n, err
}
