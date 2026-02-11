package sdk

import (
	"bytes"
	"io"
)

func readAndRestore(body *io.ReadCloser) []byte {
	if body == nil || *body == nil {
		return nil
	}
	b, _ := io.ReadAll(*body)
	_ = (*body).Close()
	*body = io.NopCloser(bytes.NewReader(b))
	return b
}
