# tdl

<img align="right" src="docs/assets/img/logo.png" height="280" alt="">

> 📥 Telegram Downloader, but more than a downloader

#### A fork for myself to use :
- using ffmpeg to get video info and split video
- add `-as-album` and `-max-album-size` flags to upload as album
- add `-max-file-size` flag to auto split video file if it is greater than the value, default is 2GB
- <s>add `-caption` flag to add custom caption header</s>
- add `-app-id` and `-app-hash` flags to use your own app id and app hash. If not set, it will use the app id and app hash of `iyear`
- add `-caption-header` and `-caption-body` and `-caption-footer` flags to add custom caption
- if set `--rm` flag, it will also remove thumbnail after uploading

#### 2025-01-27
- drop files with size 0 before upload
- send uploaded file as album when upload cancelled by user
- fix video frame rate parsing bug

#### 2025-01-30
- add `-thumb-time` flag to set thumbnail time, default is `00:00:01`
- When the sending conditions are met, the message will be sent immediately instead of waiting for all files to be uploaded.
- fix video duration bug when flag `-as-album` is not set

#### 2025-02-01
- add `-force-mp4` flag to force to upload video as `video/mp4` even if the file is not a mp4 video

#### 2025-02-10
- add logger module
- remove file only after send sucessfully

#### 2025-03-08
- add `-no-caption` flag to disable caption
- set nosoundvideo to true for all videos; see: [Fix MediaEmptyError error when sending some videos](https://github.com/LonamiWebs/Telethon/commit/ef4f9a962c6ef41b1b1905186a26c0695b1e4be2#diff-0ce168e1d5e6cf17ddbdf0b9d9d36bb5a8661e08a65a87276a3be39e7110125e)

#### 2025-03-25
- thumb file name is the same as the video file name(with extension) with extension '.thumb'

English | <a href="README_zh.md">简体中文</a>

<p>
<img src="https://img.shields.io/github/go-mod/go-version/iyear/tdl?style=flat-square" alt="">
<img src="https://img.shields.io/github/license/iyear/tdl?style=flat-square" alt="">
<img src="https://img.shields.io/github/actions/workflow/status/iyear/tdl/master.yml?branch=master&amp;style=flat-square" alt="">
<img src="https://img.shields.io/github/v/release/iyear/tdl?color=red&amp;style=flat-square" alt="">
<img src="https://img.shields.io/github/downloads/iyear/tdl/total?style=flat-square" alt="">
</p>

#### Features:
- Single file start-up
- Low resource usage
- Take up all your bandwidth
- Faster than official clients
- Download files from (protected) chats
- Forward messages with automatic fallback and message routing
- Upload files to Telegram
- Export messages/members/subscribers to JSON

## Preview

It reaches my proxy's speed limit, and the **speed depends on whether you are a premium**

![](docs/assets/img/preview.gif)

## Documentation

Please refer to the [documentation](https://docs.iyear.me/tdl/).

## Sponsors

![](https://raw.githubusercontent.com/iyear/sponsor/master/sponsors.svg)

## Contributors
<a href="https://github.com/lshcx/tdl/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=iyear/tdl&max=750&columns=20" alt="contributors"/>
</a>

## LICENSE

AGPL-3.0 License
