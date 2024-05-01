package filemanager

import (
	"strconv"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
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
