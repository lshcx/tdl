package up

import (
	"context"
	"os"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/peers"

	"github.com/iyear/tdl/core/uploader"
	"github.com/iyear/tdl/core/util/fsutil"
	"github.com/iyear/tdl/core/util/mediautil"
)

type file struct {
	file    string
	thumb   string
	mime    string
	caption string
	size    int64
	info    *mediautil.VideoInfo
}

type iter struct {
	files  []*file
	to     peers.Peer
	photo  bool
	remove bool
	delay  time.Duration

	cur  int
	err  error
	file uploader.Elem
}

func newIter(files []*file, to peers.Peer, photo, remove bool, delay time.Duration) *iter {
	return &iter{
		files:  files,
		to:     to,
		photo:  photo,
		remove: remove,
		delay:  delay,

		cur:  0,
		err:  nil,
		file: nil,
	}
}

func (i *iter) Next(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		i.err = ctx.Err()
		return false
	default:
	}

	if i.cur >= len(i.files) || i.err != nil {
		return false
	}

	// if delay is set, sleep for a while for each iteration
	if i.delay > 0 && i.cur > 0 { // skip first delay
		time.Sleep(i.delay)
	}

	cur := i.files[i.cur]
	i.cur++

	// build thumbnail
	var thumb *uploaderFile = nil
	if cur.thumb != "" {
		if !i.validThumb(cur.thumb) {
			vp := mediautil.GetVideoProcessor("")
			if vp != nil {
				vp.GenerateThumbnail(ctx, "00:00:01", cur.file, cur.thumb)
			}
		}
		thumbFile, err := os.Open(cur.thumb)
		if err == nil {
			thumb = &uploaderFile{File: thumbFile, size: 0}
		}
	}

	// build uploader file
	f, err := os.Open(cur.file)
	if err != nil {
		i.err = errors.Wrap(err, "open file")
		return false
	}
	file := &uploaderFile{File: f, size: cur.size}

	// build uploader elem
	e := &iterElem{
		file:    file,
		thumb:   thumb,
		to:      i.to,
		asPhoto: i.photo,
		remove:  i.remove,
		caption: cur.caption,
		mime:    cur.mime,
	}

	if cur.info != nil {
		e.duration = cur.info.Duration
		e.width = cur.info.Width
		e.height = cur.info.Height
		e.codec = cur.info.Codec
	}
	i.file = e

	return true
}

func (i *iter) Value() uploader.Elem {
	return i.file
}

func (i *iter) Err() error {
	return i.err
}

func (i *iter) validThumb(path string) bool {
	// 文件是否存在
	if !fsutil.PathExists(path) {
		return false
	}
	// 文件是否是图片
	mime, err := mimetype.DetectFile(path)
	if err != nil || !mediautil.IsImage(mime.String()) {
		return false
	}
	// 是否可以打开
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	return true
}
