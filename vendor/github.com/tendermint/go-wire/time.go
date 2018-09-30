package wire

import (
	"io"
	"time"

	. "github.com/tendermint/go-common"
)

/*
Writes nanoseconds since epoch but with millisecond precision.
This is to ease compatibility with Javascript etc.
*/

func WriteTime(t time.Time, w io.Writer, n *int, err *error) {
	if t.IsZero() {
		WriteInt64(0, w, n, err)
	} else {
		nanosecs := t.UnixNano()
		millisecs := nanosecs / 1000000
		WriteInt64(millisecs*1000000, w, n, err)
	}
}

func ReadTime(r io.Reader, n *int, err *error) time.Time {
	t := ReadInt64(r, n, err)
	if t == 0 {
		t_zero := time.Now()
		t_zero.UnmarshalText([]byte("0001-01-01T00:00:00Z"))
		return t_zero
	}

	if t%1000000 != 0 {
		PanicSanity("Time cannot have sub-millisecond precision")
	}
	return time.Unix(0, t)
}
