package up

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"

	"github.com/lshcx/tdl/core/util/mediautil"
	"github.com/lshcx/tdl/pkg/consts"
)

type info struct {
	imageNum int
	videoNum int
	audioNum int
	otherNum int

	imageSize     int64
	videoSize     int64
	videoDuration float64
}

func walk(ctx context.Context, paths, excludes []string) ([]*file, error) {
	files := make([]*file, 0)
	excludesMap := map[string]struct{}{
		consts.UploadThumbExt: {}, // ignore thumbnail files
	}

	for _, exclude := range excludes {
		excludesMap[exclude] = struct{}{}
	}

	for _, path := range paths {
		err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if _, ok := excludesMap[filepath.Ext(path)]; ok {
				return nil
			}

			f, err := buildFile(ctx, path)
			if err == nil && f != nil {
				files = append(files, f)
			} else {
				// Skip file if error occurs
				fmt.Printf("Warning: Skip file %s because of error: %s \n", path, err)
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return files, nil
}

func buildFile(ctx context.Context, path string) (*file, error) {
	file := &file{file: path}
	t := strings.TrimSuffix(path, filepath.Ext(path)) + consts.UploadThumbExt
	file.thumb = t

	size, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	file.size = size.Size()

	// get mime
	mime, err := mimetype.DetectFile(path)
	if err != nil {
		return nil, err
	}
	file.mime = mime.String()

	// if mime is `application/octet-stream`, and filename ends with `.ts`, then set mime to `video/mp2t`
	if file.mime == "application/octet-stream" && strings.HasSuffix(path, ".ts") {
		file.mime = "video/mp4"
	}

	if mediautil.IsVideo(file.mime) {
		file.mime = "video/mp4"
	}

	// get video info if is a video
	if mediautil.IsVideo(file.mime) {
		vp := mediautil.GetVideoProcessor(consts.FFmpegPath)
		if vp != nil {
			info, err := vp.GetVideoInfo(ctx, path)
			if err != nil {
				file.info = nil
			}
			file.info = info
		}
	}

	return file, nil
}

func stats(files []*file) *info {
	info := &info{}
	for _, f := range files {
		if mediautil.IsVideo(f.mime) {
			info.videoNum++
			info.videoSize += f.info.Size
			if f.info != nil {
				info.videoDuration += f.info.Duration
			}
		} else if mediautil.IsAudio(f.mime) {
			info.audioNum++
		} else if mediautil.IsImage(f.mime) {
			info.imageNum++
			info.imageSize += f.size
		} else {
			info.otherNum++
		}
	}
	return info
}

func handleCaption(files []*file, asAlbum bool, optCaption Caption) error {

	// build header
	header := optCaption.CaptionHeader
	footer := optCaption.CaptionFooter
	body := optCaption.CaptionBody

	// 如果header不为空，并且最后不是换行符，则添加换行符
	if header != "" && header[len(header)-1] != '\n' {
		header += "\n"
	}

	caption := ""
	if body == "" {
		if asAlbum {
			info := stats(files)
			if info.imageNum > 0 {
				body += fmt.Sprintf("【图片】: %dP %.2fGB\n", info.imageNum, float64(info.imageSize)/1024/1024/1024)
			}
			if info.videoNum > 0 {
				body += fmt.Sprintf("【视频】: %dV %.2fGB\n", info.videoNum, float64(info.videoSize)/1024/1024/1024)
				body += fmt.Sprintf("【时长】: %.2f分钟\n", info.videoDuration/60)
			}
			if info.audioNum > 0 {
				body += fmt.Sprintf("【音频】: %dA\n", info.audioNum)
			}
			if info.otherNum > 0 {
				body += fmt.Sprintf("【其他】: %d\n", info.otherNum)
			}
			caption += header + body + footer
		} else {
			// base name
			body += "【标题】：%s\n%s"
			caption += header + body + footer
		}
	} else {
		caption = header + body + footer
	}

	if asAlbum {
		for _, f := range files {
			f.caption = caption
		}
	} else {
		for _, f := range files {
			if mediautil.IsVideo(f.mime) && f.info != nil {
				tmpStr := ""
				if f.info.Size > 0 {
					tmpStr += fmt.Sprintf("【大小】：%.2fMB\n", float64(f.info.Size)/1024/1024)
				}
				if f.info.Duration > 0 {
					tmpStr += fmt.Sprintf("【时长】：%.2f分钟\n", f.info.Duration)
				}
				f.caption = fmt.Sprintf(caption, filepath.Base(f.file), tmpStr)
			} else {
				f.caption = fmt.Sprintf(caption, filepath.Base(f.file), "")
			}
		}
	}

	return nil
}

func filterFileSize(ctx context.Context, files []*file, maxFileSize float64, isRemove bool) []*file {
	filteredFiles := make([]*file, 0)
	maxSize := int64(maxFileSize * 1024 * 1024 * 1024)

	for _, f := range files {
		if f.size == 0 {
			fmt.Printf("Warning: Skip file %s because file size is 0\n", f.file)
			continue
		}

		// 如果文件大小小于等于最大文件大小，则添加到过滤后的文件列表
		if f.size <= maxSize {
			filteredFiles = append(filteredFiles, f)
			continue
		}

		// 如果不是视频文件，则跳过
		if !mediautil.IsVideo(f.mime) {
			// 如果文件不是视频，则跳过
			fmt.Printf("Warning: Skip file %s because it is not a video but size is greater than maxFileSize\n", f.file)
			continue
		}

		// 如果是视频文件，则需要分割
		vp := mediautil.GetVideoProcessor(consts.FFmpegPath)
		if vp == nil || f.info == nil {
			fmt.Printf("Warning: Skip file %s because it is a video need to be split but no video processor found\n", f.file)
			continue
		}

		// 计算分割的片段数量
		parts := (f.info.Size + maxSize - 1) / maxSize
		duration := f.info.Duration / float64(parts)
		splitFiles := make([]string, 0)

		for i := 0; i < int(parts); i++ {
			splitPath := fmt.Sprintf("%s_part%d%s", strings.TrimSuffix(f.file, filepath.Ext(f.file)), i+1, filepath.Ext(f.file))
			err := vp.SplitVideo(ctx, f.file, mediautil.SplitOptions{
				StartTime:  duration * float64(i),
				Duration:   duration,
				OutputPath: splitPath,
			})
			if err != nil {
				// 如果分割失败，清理已分割的文件
				for k := 0; k < i; k++ {
					os.Remove(splitFiles[k])
				}
				fmt.Printf("Warning: Split video %s failed: %s\n", f.file, err)
				continue
			}
			splitFiles = append(splitFiles, splitPath)
		}

		// build split files
		for _, splitPath := range splitFiles {
			f, err := buildFile(ctx, splitPath)
			if err != nil {
				fmt.Printf("Warning: Skip file %s because of error: %s \n", splitPath, err)
				continue
			}
			filteredFiles = append(filteredFiles, f)
		}

		// 如果需要删除原始文件，则删除
		if isRemove {
			os.Remove(f.file)
		}
	}

	return filteredFiles
}
