package model

import "errors"

type Song struct {
	ID             string
	ReqID          int
	SongDataBase64 string
	SongFormat     string
}

var ErrSongSavingFailure = errors.New("couldn't save song in object storage")
var ErrSongDataDecodeFailure = errors.New("error in decoding base64 song data")
