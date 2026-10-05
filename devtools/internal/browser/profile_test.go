package browser

import (
	"bytes"
	"google.golang.org/protobuf/encoding/protowire"
	"testing"
)

func TestUnknownProtoFieldsSurvivePatch(t *testing.T) {
	unknown := fieldBytes(103, []byte{0, 255, 1})
	existing := fieldBytes(3, []byte("existing authority"))
	src := append(fieldInt(2, 39), unknown...)
	src = append(src, existing...)
	out, e := Replace(src, map[protowire.Number][]byte{2: fieldInt(2, RootVersion)})
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(out, unknown) || !bytes.Contains(out, existing) || bytes.Contains(out, fieldInt(2, 39)) {
		t.Fatal("patch lost existing bytes or retained stale version")
	}
	if _, e = Replace([]byte{0xff}, nil); e == nil {
		t.Fatal("malformed protobuf accepted")
	}
}
