package binding

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestDecodeRejectsUnexpectedSerializationAndHeaders(t *testing.T) {
	for _, doc := range []string{"", "e30.payload.sig", base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","unexpected":true}`)) + "..AA"} {
		if _, _, _, _, e := Decode(doc); e == nil {
			t.Fatalf("accepted %q", doc)
		}
	}
	if _, _, e := Sign(nil, nil, time.Time{}, 0); e == nil {
		t.Fatal("nil credentials accepted")
	}
}
