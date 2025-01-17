package uploader

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"golang.org/x/sync/errgroup"

	"github.com/lshcx/tdl/core/tmedia"
	"github.com/lshcx/tdl/core/util/mediautil"
)

// MaxPartSize refer to https://core.telegram.org/api/files#uploading-files
const MaxPartSize = 512 * 1024

type mediaBinding struct {
	index int
	elem  Elem
	media tg.InputMediaClass
}

type Uploader struct {
	opts       Options
	albumMedia []mediaBinding
	mu         sync.Mutex
}

type Options struct {
	Client       *tg.Client
	Threads      int
	Limit        int
	Iter         Iter
	Progress     Progress
	AsAlbum      bool
	MaxAlbumSize int
}

func New(o Options) *Uploader {
	return &Uploader{opts: o}
}

func (u *Uploader) Upload(ctx context.Context) error {
	wg, wgctx := errgroup.WithContext(ctx)
	wg.SetLimit(u.opts.Limit)

	u.albumMedia = make([]mediaBinding, 0)
	index := 0
	for u.opts.Iter.Next(wgctx) {
		elem := u.opts.Iter.Value()
		id := index
		index++

		// use currentElem and currentID to avoid race condition
		// I don't know whether it's necessary. It's from claude-3.5-sonnet.
		currentElem := elem
		currentID := id

		wg.Go(func() (rerr error) {
			u.opts.Progress.OnAdd(currentElem)
			defer func() { u.opts.Progress.OnDone(currentElem, rerr) }()

			media, err := u.uploadFile(wgctx, currentElem)
			if err != nil {
				// canceled by user, so we directly return error to stop all
				if errors.Is(err, context.Canceled) {
					return errors.Wrap(err, "upload canceled by user")
				}

				// don't return error, just log it
				fmt.Printf("Error: upload file %s failed: %v\n", currentElem.File().Name(), err)
			}

			u.mu.Lock()
			u.albumMedia = append(u.albumMedia, mediaBinding{
				index: currentID,
				elem:  currentElem,
				media: media,
			})
			u.mu.Unlock()

			return nil
		})
	}

	if err := u.opts.Iter.Err(); err != nil {
		return errors.Wrap(err, "iter error")
	}

	if err := wg.Wait(); err != nil {
		return errors.Wrap(err, "wait uploader")
	}

	// sort albumMedia by index
	sort.Slice(u.albumMedia, func(i, j int) bool {
		return u.albumMedia[i].index < u.albumMedia[j].index
	})

	if u.opts.AsAlbum && len(u.albumMedia) > 0 {
		return u.sendMultiMedia(ctx, u.albumMedia)
	} else {
		for _, mb := range u.albumMedia {
			if err := u.sendSingleMedia(ctx, mb); err != nil {
				return errors.Wrap(err, "send single media")
			}
		}
	}

	return nil
}

func (u *Uploader) uploadFile(ctx context.Context, elem Elem) (tg.InputMediaClass, error) {
	// check if context is canceled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	up := uploader.NewUploader(u.opts.Client).
		WithPartSize(MaxPartSize).
		WithThreads(u.opts.Threads).
		WithProgress(&wrapProcess{
			elem:    elem,
			process: u.opts.Progress,
		})

	// upload file
	f, err := up.Upload(ctx, uploader.NewUpload(elem.File().Name(), elem.File(), elem.File().Size()))
	if err != nil {
		return nil, errors.Wrap(err, "upload file")
	}

	// build attributes
	attributes := []tg.DocumentAttributeClass{
		&tg.DocumentAttributeFilename{
			FileName: elem.File().Name(),
		},
	}

	// get mine
	mime := elem.Mime()

	// build media based on mime
	var media tg.InputMediaClass
	switch {
	case mediautil.IsImage(mime) && mime != "image/webp" && elem.AsPhoto():
		photo := &tg.InputMediaUploadedPhoto{
			File: f,
		}
		photo.SetFlags()
		media = photo
	case mediautil.IsVideo(mime):

		videoAttributes := &tg.DocumentAttributeVideo{
			SupportsStreaming: true,
		}
		if elem.Duration() > 0 {
			videoAttributes.Duration = elem.Duration()
		}
		if elem.Width() > 0 {
			videoAttributes.W = elem.Width()
		}
		if elem.Height() > 0 {
			videoAttributes.H = elem.Height()
		}
		videoAttributes.SetFlags()
		attributes = append(attributes, videoAttributes)
		doc := &tg.InputMediaUploadedDocument{
			File:       f,
			MimeType:   mime,
			Attributes: attributes,
		}
		// set thumbnail if has
		if thumb, ok := elem.Thumb(); ok {
			if thumb, err := uploader.NewUploader(u.opts.Client).
				FromReader(ctx, thumb.Name(), thumb); err == nil {
				doc.Thumb = thumb
			}
		}

		doc.SetFlags()
		media = doc
	case mediautil.IsAudio(mime):
		attributes = append(attributes, &tg.DocumentAttributeAudio{})
		audio := &tg.InputMediaUploadedDocument{
			File:       f,
			MimeType:   mime,
			Attributes: attributes,
		}
		audio.SetFlags()
		media = audio
	default:
		doc := &tg.InputMediaUploadedDocument{
			File:       f,
			MimeType:   mime,
			Attributes: attributes,
		}

		doc.SetFlags()
		media = doc
	}

	// test
	req := &tg.MessagesSendMediaRequest{
		Peer:     elem.To(),
		Media:    media,
		Message:  elem.Caption(),
		RandomID: time.Now().UnixNano(),
		Silent:   false,
	}
	req.SetFlags()

	_, err = u.opts.Client.MessagesSendMedia(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "send single media")
	}

	// Uploads a media file to a chat, without sending it, returning only a MessageMedia
	// constructor that can be used to later send the file to multiple chats, without
	// reuploading it every time. ref: https://core.telegram.org/api/files#albums-grouped-media

	// note that they must be separately uploaded using messages uploadMedia first,
	// using raw inputMediaUploaded* constructors is not supported.
	// ref: https://core.telegram.org/method/messages.sendMultiMedia
	msgMedia, err := u.opts.Client.MessagesUploadMedia(ctx, &tg.MessagesUploadMediaRequest{
		Peer:  elem.To(),
		Media: media,
	})
	if err != nil {
		return nil, errors.Wrap(err, "message upload media")
	}

	// convert msgMedia to InputMediaClass
	inputMedia, ok := tmedia.ConvInputMedia(msgMedia)
	if !ok {
		return nil, errors.New("convert msgMedia to InputMediaClass")
	}

	return inputMedia, nil
}

func (u *Uploader) sendSingleMedia(ctx context.Context, mb mediaBinding) error {
	media := mb.media
	elem := mb.elem

	req := &tg.MessagesSendMediaRequest{
		Peer:     elem.To(),
		Media:    media,
		Message:  elem.Caption(),
		RandomID: time.Now().UnixNano(),
		Silent:   false,
	}
	req.SetFlags()

	_, err := u.opts.Client.MessagesSendMedia(ctx, req)
	if err != nil {
		return errors.Wrap(err, "send single media")
	}

	return nil
}

func (u *Uploader) sendMultiMedia(ctx context.Context, mbs []mediaBinding) error {

	hasCaption := true
	// build inputSingleMedia list
	inputSingleMedias := make([]tg.InputSingleMedia, 0, len(mbs))
	for _, mb := range mbs {
		single := tg.InputSingleMedia{
			Media:    mb.media,
			RandomID: time.Now().UnixNano(),
		}
		if hasCaption {
			single.Message = mb.elem.Caption()
			hasCaption = false
		}
		single.SetFlags()
		inputSingleMedias = append(inputSingleMedias, single)
	}

	// split into batches and send
	maxAlbumSize := min(u.opts.MaxAlbumSize, 10)
	for i := 0; i < len(inputSingleMedias); i += maxAlbumSize {
		batch := inputSingleMedias[i:min(i+maxAlbumSize, len(inputSingleMedias))]
		req := &tg.MessagesSendMultiMediaRequest{
			Peer:       mbs[0].elem.To(),
			MultiMedia: batch,
			Silent:     false,
		}
		req.SetFlags()

		_, err := u.opts.Client.MessagesSendMultiMedia(ctx, req)
		if err != nil {
			return errors.Wrap(err, "send multi media batch failed at index "+strconv.Itoa(i))
		}
	}

	return nil
}
