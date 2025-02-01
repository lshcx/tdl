package up

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
	"github.com/spf13/viper"
	"go.uber.org/multierr"

	"github.com/lshcx/tdl/core/dcpool"
	"github.com/lshcx/tdl/core/storage"
	"github.com/lshcx/tdl/core/tclient"
	"github.com/lshcx/tdl/core/uploader"
	"github.com/lshcx/tdl/core/util/tutil"
	"github.com/lshcx/tdl/pkg/consts"
	"github.com/lshcx/tdl/pkg/prog"
	"github.com/lshcx/tdl/pkg/utils"
)

type Caption struct {
	CaptionHeader string
	CaptionBody   string
	CaptionFooter string
}

type Options struct {
	Chat         string
	Paths        []string
	Excludes     []string
	Remove       bool
	Photo        bool
	AsAlbum      bool
	MaxAlbumSize int
	ThumbTime    string
	ForceMp4     bool
	MaxFileSize  float64 // GB
	Caption      Caption
}

func Run(ctx context.Context, c *telegram.Client, kvd storage.Storage, opts Options) (rerr error) {
	files, err := walk(ctx, opts.Paths, opts.Excludes, opts.ForceMp4)
	if err != nil {
		return errors.Wrap(err, "walk")
	}

	files = filterFileSize(ctx, files, opts.MaxFileSize, opts.Remove, opts.ForceMp4)

	if err := handleCaption(files, opts.AsAlbum, opts.Caption); err != nil {
		return errors.Wrap(err, "handle caption")
	}

	// show files
	for _, f := range files {
		fmt.Printf("File: %s, Size: %d, Mime: %s, Thumb: %s, Info: %+v\n", f.file, f.size, f.mime, f.thumb, f.info)
	}

	color.Blue("Files count: %d", len(files))

	pool := dcpool.NewPool(c,
		int64(viper.GetInt(consts.FlagPoolSize)),
		tclient.NewDefaultMiddlewares(ctx, viper.GetDuration(consts.FlagReconnectTimeout))...)
	defer multierr.AppendInvoke(&rerr, multierr.Close(pool))

	manager := peers.Options{Storage: storage.NewPeers(kvd)}.Build(pool.Default(ctx))

	to, err := resolveDestPeer(ctx, manager, opts.Chat)
	if err != nil {
		return errors.Wrap(err, "get target peer")
	}

	upProgress := prog.New(utils.Byte.FormatBinaryBytes)
	upProgress.SetNumTrackersExpected(len(files))
	prog.EnablePS(ctx, upProgress)

	options := uploader.Options{
		Client:       pool.Default(ctx),
		Threads:      viper.GetInt(consts.FlagThreads),
		Limit:        viper.GetInt(consts.FlagLimit),
		Iter:         newIter(files, to, opts.Photo, opts.Remove, viper.GetDuration(consts.FlagDelay), opts.ThumbTime),
		Progress:     newProgress(upProgress),
		AsAlbum:      opts.AsAlbum,
		MaxAlbumSize: opts.MaxAlbumSize,
	}

	up := uploader.New(options)

	go upProgress.Render()
	defer prog.Wait(ctx, upProgress)

	return up.Upload(ctx)
}

func resolveDestPeer(ctx context.Context, manager *peers.Manager, chat string) (peers.Peer, error) {
	if chat == "" {
		return manager.FromInputPeer(ctx, &tg.InputPeerSelf{})
	}

	return tutil.GetInputPeer(ctx, manager, chat)
}
