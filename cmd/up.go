package cmd

import (
	"context"

	"github.com/gotd/td/telegram"
	"github.com/spf13/cobra"

	"github.com/lshcx/tdl/app/up"
	"github.com/lshcx/tdl/core/logctx"
	"github.com/lshcx/tdl/core/storage"
)

func NewUpload() *cobra.Command {
	var opts up.Options

	cmd := &cobra.Command{
		Use:     "upload",
		Aliases: []string{"up"},
		Short:   "Upload anything to Telegram",
		GroupID: groupTools.ID,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tRun(cmd.Context(), func(ctx context.Context, c *telegram.Client, kvd storage.Storage) error {
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
	cmd.Flags().IntVar(&opts.MaxFileSize, "max-file-size", 2, "max file size(GB), if the file size is greater than this value, it will be split into multiple files")

	// completion and validation
	_ = cmd.MarkFlagRequired(path)

	return cmd
}
