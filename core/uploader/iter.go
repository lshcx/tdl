package uploader

import (
	"context"
	"io"

	"github.com/gotd/td/tg"
)

type Iter interface {
	Next(ctx context.Context) bool
	Value() Elem
	Err() error
}

type File interface {
	io.ReadSeeker
	Name() string
	Size() int64
}

type Elem interface {
	File() File
	Thumb() (string, bool)
	To() tg.InputPeerClass
	AsPhoto() bool
	Mime() string
	Duration() float64
	Width() int
	Height() int
	Codec() string
	Caption() string
	DoRemove() error
}
