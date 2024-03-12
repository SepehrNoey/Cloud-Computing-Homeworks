package model

import (
	"errors"
)

type Song struct {
	ID             string `json:"id,omitempty"`
	ReqID          int    `json:"req_id"`
	SongDataBinary []byte `json:"song_data_binary"`
	SongFormat     string `json:"song_format"`
}

var ErrSongSavingFailure = errors.New("couldn't save song in object storage")
var ErrSongDataDecodeFailure = errors.New("error in decoding base64 song data")
