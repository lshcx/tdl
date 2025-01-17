package up

import (
	"os"
	"path/filepath"

	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"

	"github.com/lshcx/tdl/core/uploader"
)

type uploaderFile struct {
	*os.File
	size int64
}

func (u *uploaderFile) Name() string {
	return filepath.Base(u.File.Name())
}

func (u *uploaderFile) Size() int64 {
	return u.size
}

type iterElem struct {
	file  *uploaderFile
	thumb *uploaderFile
	to    peers.Peer

	asPhoto  bool
	remove   bool
	caption  string
	mime     string
	duration float64
	width    int
	height   int
	codec    string
}

func (e *iterElem) File() uploader.File {
	return e.file
}

func (e *iterElem) Thumb() (uploader.File, bool) {
	if e.thumb == nil {
		return nil, false
	}
	return e.thumb, true
}

func (e *iterElem) To() tg.InputPeerClass {
	return e.to.InputPeer()
}

func (e *iterElem) AsPhoto() bool {
	return e.asPhoto
}

func (e *iterElem) Caption() string {
	return e.caption
}

func (e *iterElem) Mime() string {
	return e.mime
}

func (e *iterElem) Duration() float64 {
	return e.duration
}

func (e *iterElem) Width() int {
	return e.width
}

func (e *iterElem) Height() int {
	return e.height
}

func (e *iterElem) Codec() string {
	return e.codec
}
