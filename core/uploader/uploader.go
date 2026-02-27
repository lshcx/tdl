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
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/lshcx/tdl/core/tmedia"
	"github.com/lshcx/tdl/core/util/mediautil"
	"github.com/lshcx/tdl/pkg/logger"
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
	albumIndex := 0
	hasCaption := true

	// 用于跟踪是否被用户取消
	var canceled bool

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
				if errors.Is(err, context.Canceled) {
					canceled = true
					// 不立即返回错误，让已上传的文件能被处理
					return nil
				}

				// don't return error, just log it
				logger.Error("Error: upload file", zap.String("file: ", currentElem.File().Name()), zap.Error(err))
				fmt.Printf("Error: upload file %s failed: %v\n", currentElem.File().Name(), err)
				return nil
			}

			u.mu.Lock()
			defer u.mu.Unlock()

			// insert media to u.albumMedia by index
			u.albumMedia = append(u.albumMedia, mediaBinding{
				index: currentID,
				elem:  currentElem,
				media: media,
			})
			sort.Slice(u.albumMedia, func(i, j int) bool {
				return u.albumMedia[i].index < u.albumMedia[j].index
			})

			// 如果前maxAlbumSize个元素的index是连续的并且第一个元素的index是albumIndex，则取出前maxAlbumSize个元素
			if len(u.albumMedia) >= u.opts.MaxAlbumSize && u.albumMedia[0].index == albumIndex && u.albumMedia[0].index+u.opts.MaxAlbumSize-1 == u.albumMedia[u.opts.MaxAlbumSize-1].index {
				albumMedia := u.albumMedia[:u.opts.MaxAlbumSize]
				u.albumMedia = u.albumMedia[u.opts.MaxAlbumSize:]
				if err := u.send(albumMedia, hasCaption); err != nil {
					// don't return error, just log it
					logger.Error("Error: send uploaded files", zap.Error(err))
					fmt.Printf("Error: send uploaded files failed: %v\n", err)
					return nil
				}
				if hasCaption {
					hasCaption = false
				}
				albumIndex += u.opts.MaxAlbumSize
			}

			return nil
		})
	}

	// 检查迭代器错误
	if err := u.opts.Iter.Err(); err != nil {
		if !errors.Is(err, context.Canceled) {
			return errors.Wrap(err, "iter error")
		}
		canceled = true
	}

	// 等待所有上传任务完成
	if err := wg.Wait(); err != nil {
		if !errors.Is(err, context.Canceled) {
			return errors.Wrap(err, "wait uploader")
		}
		canceled = true
	}

	// 发送已上传的文件
	if err := u.send(u.albumMedia, hasCaption); err != nil {
		return errors.Wrap(err, "send uploaded files")
	}

	// 如果是用户取消，最后再返回取消错误
	if canceled {
		return errors.New("upload canceled by user")
	}

	return nil
}

func (u *Uploader) send(mbs []mediaBinding, hasCaption bool) error {
	if len(mbs) > 0 {
		// 创建新的 context 用于发送
		sendCtx := context.Background()

		// 排序已上传的媒体
		sort.Slice(mbs, func(i, j int) bool {
			return mbs[i].index < mbs[j].index
		})

		// 发送已上传的文件
		if u.opts.AsAlbum {
			if err := u.sendMultiMedia(sendCtx, mbs, hasCaption); err != nil {
				return errors.Wrap(err, "send multi media")
			}
		} else {
			for _, mb := range mbs {
				if err := u.sendSingleMedia(sendCtx, mb); err != nil {
					return errors.Wrap(err, "send single media")
				}
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
			File:         f,
			MimeType:     mime,
			Attributes:   attributes,
			NosoundVideo: true,
		}
		// set thumbnail if has
		if thumbPath, ok := elem.Thumb(); ok {
			if thumb, err := uploader.NewUploader(u.opts.Client).
				FromPath(ctx, thumbPath); err == nil {
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

	if err := elem.DoRemove(); err != nil {
		return errors.Wrap(err, "remove file")
	}

	return nil
}

func (u *Uploader) sendMultiMedia(ctx context.Context, mbs []mediaBinding, hasCaption bool) error {

	isFirst := true
	// build inputSingleMedia list
	inputSingleMedias := make([]tg.InputSingleMedia, 0, len(mbs))
	elems := make([]Elem, 0, len(mbs))
	for _, mb := range mbs {
		single := tg.InputSingleMedia{
			Media:    mb.media,
			RandomID: time.Now().UnixNano(),
		}
		if hasCaption && isFirst {
			single.Message = mb.elem.Caption()
			isFirst = false
		}
		single.SetFlags()
		inputSingleMedias = append(inputSingleMedias, single)
		elems = append(elems, mb.elem)

		fmt.Printf("Build InputSingleMedia\n")
	}

	// split into batches and send
	maxAlbumSize := min(u.opts.MaxAlbumSize, 10)
	fmt.Printf("maxAlbumSize: %d\n", maxAlbumSize)
	for i := 0; i < len(inputSingleMedias); i += maxAlbumSize {
		fmt.Printf("Index: %d, total: %d\n", i, len(inputSingleMedias))
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
		fmt.Printf("Success\n")
	}
	fmt.Printf("Remove\n")
	for _, elem := range elems {
		if err := elem.DoRemove(); err != nil {
			return errors.Wrap(err, "remove file")
		}
	}

	return nil
}
