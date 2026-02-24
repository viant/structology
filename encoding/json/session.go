package json

import (
	"bytes"
	"sync"
)

type MarshalInterceptor func() ([]byte, error)

type UnmarshalInterceptor func(dst interface{}, codec Codec, options ...interface{}) error

type MarshalInterceptors map[string]MarshalInterceptor

type UnmarshalInterceptors map[string]UnmarshalInterceptor

var bufferPool = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, 256)) }}

type MarshalSession struct {
	Buffer       *bytes.Buffer
	Options      Options
	Interceptors MarshalInterceptors
	path         pathState
}

type UnmarshalSession struct {
	Options      Options
	Interceptors UnmarshalInterceptors
	path         pathState
}

func NewMarshalSession(options Options) *MarshalSession { return newMarshalSession(options) }

func newMarshalSession(options Options) *MarshalSession {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return &MarshalSession{Buffer: buf, Options: options, Interceptors: MarshalInterceptors{}}
}

func (s *MarshalSession) release() {
	if s == nil || s.Buffer == nil {
		return
	}
	bufferPool.Put(s.Buffer)
	s.Buffer = nil
}

func (s *MarshalSession) Release() { s.release() }

func newUnmarshalSession(options Options) *UnmarshalSession {
	return &UnmarshalSession{Options: options, Interceptors: UnmarshalInterceptors{}}
}

func NewUnmarshalSession(options Options) *UnmarshalSession { return newUnmarshalSession(options) }

func (s *MarshalSession) PushField(name string)   { s.path.pushField(name) }
func (s *MarshalSession) PushIndex(index int)     { s.path.pushIndex(index) }
func (s *MarshalSession) PopPath()                { s.path.pop() }
func (s *MarshalSession) PathRef() PathRef        { return s.path.ref() }
func (s *UnmarshalSession) PushField(name string) { s.path.pushField(name) }
func (s *UnmarshalSession) PushIndex(index int)   { s.path.pushIndex(index) }
func (s *UnmarshalSession) PopPath()              { s.path.pop() }
func (s *UnmarshalSession) PathRef() PathRef      { return s.path.ref() }

type pathState struct {
	segments []PathSegment
}

func (p *pathState) pushField(name string) {
	p.segments = append(p.segments, PathSegment{Kind: SegmentField, Field: name})
}

func (p *pathState) pushIndex(index int) {
	p.segments = append(p.segments, PathSegment{Kind: SegmentIndex, Index: index})
}

func (p *pathState) pop() {
	if len(p.segments) == 0 {
		return
	}
	p.segments = p.segments[:len(p.segments)-1]
}

func (p *pathState) ref() PathRef {
	cp := make([]PathSegment, len(p.segments))
	copy(cp, p.segments)
	return PathRef{segments: cp, depth: len(cp)}
}
