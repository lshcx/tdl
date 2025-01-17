package mediautil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/go-faster/errors"
)

// VideoInfo 存储视频文件的基本信息
type VideoInfo struct {
	FilePath  string  `json:"filePath"`  // 文件路径
	Duration  float64 `json:"duration"`  // 视频时长（秒）
	Width     int     `json:"width"`     // 视频宽度
	Height    int     `json:"height"`    // 视频高度
	Bitrate   int64   `json:"bitrate"`   // 比特率
	Codec     string  `json:"codec"`     // 视频编码
	FrameRate float64 `json:"frameRate"` // 帧率
	Size      int64   `json:"size"`      // 文件大小（字节）
	Thumbnail string  `json:"thumbnail"` // 缩略图
}

// SplitOptions 视频分割选项
type SplitOptions struct {
	StartTime  float64 `json:"startTime"`  // 开始时间（秒）
	Duration   float64 `json:"duration"`   // 分割时长（秒）
	OutputPath string  `json:"outputPath"` // 输出路径
}

// VideoProcessor 视频处理器
type VideoProcessor struct {
	ffmpegPath string
}

var (
	instance *VideoProcessor
	once     sync.Once
)

func GetVideoProcessor(ffmpegPath string) *VideoProcessor {
	once.Do(func() {
		ins, err := newVideoProcessor(ffmpegPath)
		if err != nil {
			instance = nil
		}
		instance = ins
	})

	return instance
}

// newVideoProcessor 创建新的视频处理器
func newVideoProcessor(ffmpegPath string) (*VideoProcessor, error) {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	// 检查 ffmpeg 是否可用
	cmd := exec.Command(ffmpegPath, "-version")
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrap(err, "check ffmpeg")
	}

	return &VideoProcessor{
		ffmpegPath: ffmpegPath,
	}, nil
}

// GetVideoInfo 获取视频信息
func (p *VideoProcessor) GetVideoInfo(ctx context.Context, filepath string) (*VideoInfo, error) {
	args := []string{
		"-hide_banner",
		"-i", filepath,
	}

	cmd := exec.CommandContext(ctx, p.ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// FFmpeg 在获取信息时会返回错误码1，但仍会输出信息到stderr
		outputStr := string(output)
		info := &VideoInfo{
			FilePath: filepath,
		}

		// 解析时长
		if dur := extractValue(outputStr, "Duration: ", ","); dur != "" {
			info.Duration = parseTime(dur)
		}

		// 解析视频信息
		if stream := extractValue(outputStr, "Stream #0:0", "Stream #0:1"); stream != "" {
			// 解析分辨率
			if res := extractValue(stream, ", ", " ["); res != "" {
				fmt.Sscanf(res, "%dx%d", &info.Width, &info.Height)
			} else {
				info.Width = -1
				info.Height = -1
			}
			// 解析编码
			if codec := extractValue(stream, "Video: ", ","); codec != "" {
				info.Codec = codec
			} else {
				info.Codec = ""
			}
			// 解析帧率
			if fps := extractValue(stream, " fps,", " "); fps != "" {
				fmt.Sscanf(fps, "%f", &info.FrameRate)
			} else {
				info.FrameRate = -1
			}
		} else {
			info.Width = -1
			info.Height = -1
			info.Codec = ""
			info.FrameRate = -1
		}

		// 解析比特率
		if bitrate := extractValue(outputStr, "bitrate: ", " kb/s"); bitrate != "" {
			fmt.Sscanf(bitrate, "%d", &info.Bitrate)
		} else {
			info.Bitrate = -1
		}

		// 获取文件大小
		if stat, err := os.Stat(filepath); err == nil {
			info.Size = stat.Size()
		} else {
			info.Size = -1
		}

		return info, nil
	}
	return nil, errors.Wrap(err, "get video info")
}

// SplitVideo 分割视频
func (p *VideoProcessor) SplitVideo(ctx context.Context, inputPath string, options SplitOptions) error {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return errors.Wrap(err, "input file does not exist")
	}

	// 创建输出目录
	if err := os.MkdirAll(options.OutputPath, 0755); err != nil {
		return errors.Wrap(err, "create output directory")
	}

	args := []string{
		"-i", inputPath,
		"-ss", fmt.Sprintf("%.3f", options.StartTime),
	}

	if options.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%.3f", options.Duration))
	}

	// 使用 copy 模式，保持原视频质量
	args = append(args,
		"-c", "copy",
		"-avoid_negative_ts", "make_zero",
		"-y", // 覆盖已存在的文件
		options.OutputPath,
	)

	cmd := exec.CommandContext(ctx, p.ffmpegPath, args...)
	if _, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrap(err, "split video")
	}

	return nil
}

// 获取视频缩略图
func (p *VideoProcessor) GenerateThumbnail(ctx context.Context, time, inputPath string, outputPath string) error {
	args := []string{
		"-hide_banner",
		"-i", inputPath,
		"-ss", time,
		"-frames:v", "1",
		"-f", "image2",
		"-c:v", "mjpeg",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, p.ffmpegPath, args...)
	if _, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrap(err, "get thumbnail")
	}
	return nil
}

// 辅助函数：从文本中提取值
func extractValue(text, prefix, suffix string) string {
	if start := strings.Index(text, prefix); start != -1 {
		start += len(prefix)
		if suffix == "" {
			return strings.TrimSpace(text[start:])
		}
		if end := strings.Index(text[start:], suffix); end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	return ""
}

// 辅助函数：解析时间格式 (HH:MM:SS.ms)
func parseTime(timeStr string) float64 {
	var hours, minutes, seconds float64
	var milliseconds float64
	fmt.Sscanf(timeStr, "%f:%f:%f.%f", &hours, &minutes, &seconds, &milliseconds)
	return hours*3600 + minutes*60 + seconds + milliseconds/100
}