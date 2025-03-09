package cmd

import (
	"context"

	"github.com/gotd/td/telegram"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/lshcx/tdl/app/up"
	"github.com/lshcx/tdl/core/logctx"
	"github.com/lshcx/tdl/core/storage"
	"github.com/lshcx/tdl/pkg/logger"
)

func NewUpload() *cobra.Command {
	var opts up.Options

	cmd := &cobra.Command{
		Use:     "upload",
		Aliases: []string{"up"},
		Short:   "Upload anything to Telegram",
		GroupID: groupTools.ID,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Init(logger.Options{
				Level:      "info",
				Filename:   "tdl.log",
				MaxSize:    100,
				MaxBackups: 5,
				MaxAge:     30,
				Console:    false,
			})
			defer logger.Sync()

			return tRun(cmd.Context(), func(ctx context.Context, c *telegram.Client, kvd storage.Storage) (err error) {
				defer func() {
					if err != nil {
						logger.Error("error occered during uploading: ", zap.Error(err))
					} else {
						logger.Info("uploading successfully")
					}
				}()
				return up.Run(logctx.Named(ctx, "up"), c, kvd, opts)
			})
		},
	}

	const (
		_chat = "chat"
		path  = "path"
	)
	cmd.Flags().StringVarP(&opts.Chat, _chat, "c", "", "chat id or domain, and empty means 'Saved Messages'")
	cmd.Flags().StringSliceVarP(&opts.Paths, path, "p", []string{}, "dirs or files")
	cmd.Flags().StringSliceVarP(&opts.Excludes, "excludes", "e", []string{}, "exclude the specified file extensions")
	cmd.Flags().BoolVar(&opts.Remove, "rm", false, "remove the uploaded files after uploading")
	cmd.Flags().BoolVar(&opts.Photo, "photo", false, "upload the image as a photo instead of a file")
	cmd.Flags().BoolVar(&opts.AsAlbum, "as-album", false, "upload as an album")
	cmd.Flags().IntVar(&opts.MaxAlbumSize, "max-album-size", 10, "max album size, only works when --as-album is true")
	cmd.Flags().StringVar(&opts.Caption.CaptionHeader, "caption", "", "custom caption header(end with \\n)")
	cmd.Flags().StringVar(&opts.Caption.CaptionBody, "caption-body", "", "custom caption body(end with \\n)")
	cmd.Flags().StringVar(&opts.Caption.CaptionFooter, "caption-footer", "", "custom caption footer")
	cmd.Flags().Float64Var(&opts.MaxFileSize, "max-file-size", 2, "max file size(GB), if the file size is greater than this value, it will be split into multiple files")
	cmd.Flags().StringVar(&opts.ThumbTime, "thumb-time", "00:00:01", "thumbnail time")
	cmd.Flags().BoolVar(&opts.ForceMp4, "force-mp4", false, "force to convert video to mp4")
	cmd.Flags().BoolVar(&opts.Caption.NoCaption, "no-caption", false, "no caption")

	// completion and validation
	_ = cmd.MarkFlagRequired(path)

	return cmd
}
