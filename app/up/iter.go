package up

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/peers"

	"github.com/lshcx/tdl/core/uploader"
	"github.com/lshcx/tdl/core/util/fsutil"
	"github.com/lshcx/tdl/core/util/mediautil"
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
	files     []*file
	to        peers.Peer
	photo     bool
	remove    bool
	delay     time.Duration
	thumbTime string

	cur  int
	err  error
	file uploader.Elem
}

func newIter(files []*file, to peers.Peer, photo, remove bool, delay time.Duration, thumbTime string) *iter {
	return &iter{
		files:     files,
		to:        to,
		photo:     photo,
		remove:    remove,
		delay:     delay,
		thumbTime: thumbTime,

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
	thumb := ""
	if cur.thumb != "" {
		if !i.validThumb(cur.thumb) {
			vp := mediautil.GetVideoProcessor("")
			if vp != nil {

				// get thumb time and transform to float64
				thumbTimeF64, err := timeToFloat(i.thumbTime)
				if err != nil {
					thumbTimeF64 = 0
				}

				// generate thumbnail with specified time if the time is small than cur.info.Duration
				if i.thumbTime != "" && cur.info != nil && cur.info.Duration > thumbTimeF64 {
					vp.GenerateThumbnail(ctx, i.thumbTime, cur.file, cur.thumb)
				} else {
					vp.GenerateThumbnail(ctx, "00:00:01", cur.file, cur.thumb)
				}
			}
		}

		if i.validThumb(cur.thumb) {
			thumb = cur.thumb
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

func timeToFloat(timeStr string) (float64, error) {
	// 将格式从 "00:00:15" 转换为 "0h0m15s" 格式
	h, m, s := 0, 0, 0
	_, err := fmt.Sscanf(timeStr, "%02d:%02d:%02d", &h, &m, &s)
	if err != nil {
		return 0, err
	}

	durationStr := fmt.Sprintf("%dh%dm%ds", h, m, s)
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0, err
	}

	return duration.Seconds(), nil
}
