package yenc

import "errors"

var ErrInvalidFormat = errors.New("not a valid yEncode formatted data stream")
var ErrDataCorruption = errors.New("data corruption detected")
var ErrRejectPrefixData = errors.New("yEncode not strated at the beginning of the data stream")
var ErrBufferTooSmall = errors.New("buffer too small")
var ErrWrtingTooMuch = errors.New("written data exceeds indicated size")
