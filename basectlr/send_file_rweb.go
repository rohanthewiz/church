package basectlr

import (
	"bytes"
	"io"
	"net/http"
	"net/url"

	"github.com/rohanthewiz/rweb"
)

func SendFileRWeb(ctx rweb.Context, filename string, body []byte) error {
	ctx.Response().Header().Set("Content-Type", "application/octet-stream")
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename="+url.QueryEscape(filename))
	ctx.Response().Header().Set("x-filename", url.QueryEscape(filename))
	ctx.Response().Header().Set("Content-Description", "File Transfer")
	ctx.Response().Header().Set("Content-Transfer-Encoding", "binary")
	ctx.Response().Header().Set("Expires", "0")
	ctx.Response().Header().Set("Cache-Control", "must-revalidate")
	ctx.Response().Header().Set("Pragma", "public")
	ctx.Response().Header().Set("Access-Control-Expose-Headers", "x-filename")

	ctx.Response().WriteHeader(http.StatusOK)
	_, err := io.Copy(ctx.Response(), bytes.NewReader(body))
	return err
}

func SendAudioFileRWeb(ctx rweb.Context, filename string, body []byte) error {
	ctx.Response().Header().Set("Content-Type", "audio/mpeg")
	ctx.Response().Header().Set("Content-Disposition", "inline; filename="+url.QueryEscape(filename))
	ctx.Response().Header().Set("x-filename", url.QueryEscape(filename))
	ctx.Response().Header().Set("Content-Description", "File Transfer")
	ctx.Response().Header().Set("Content-Transfer-Encoding", "binary")
	ctx.Response().Header().Set("Expires", "0")
	ctx.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Response().Header().Set("Pragma", "public")
	ctx.Response().Header().Set("Access-Control-Expose-Headers", "x-filename")

	ctx.Response().WriteHeader(http.StatusOK)
	_, err := io.Copy(ctx.Response(), bytes.NewReader(body))
	return err
}