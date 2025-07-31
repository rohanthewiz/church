package basectlr

import (
	"net/url"

	"github.com/rohanthewiz/rweb"
)

func SendFileRWeb(ctx rweb.Context, filename string, body []byte) error {
	return rweb.File(ctx, filename, body)
}

func SendAudioFileRWeb(ctx rweb.Context, filename string, body []byte) error {
	ctx.Response().SetHeader("Content-Type", "audio/mpeg")
	ctx.Response().SetHeader("Content-Disposition", "inline; filename="+url.QueryEscape(filename))
	ctx.Response().SetHeader("x-filename", url.QueryEscape(filename))
	ctx.Response().SetHeader("Content-Description", "File Transfer")
	ctx.Response().SetHeader("Content-Transfer-Encoding", "binary")
	ctx.Response().SetHeader("Expires", "0")
	ctx.Response().SetHeader("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Response().SetHeader("Pragma", "public")
	ctx.Response().SetHeader("Access-Control-Expose-Headers", "x-filename")

	return ctx.Bytes(body)
}