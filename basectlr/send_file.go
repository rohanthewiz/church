package basectlr

import (
	"bytes"
	"net/http"
	"net/url"

	"github.com/labstack/echo"
)

func SendFile(c echo.Context, filename string, body []byte) error {
	c.Response().Header().Set("Content-Type", "application/octet-stream")
	c.Response().Header().Set("Content-Disposition", "attachment; filename="+url.QueryEscape(filename))
	c.Response().Header().Set("x-filename", url.QueryEscape(filename))
	c.Response().Header().Set("Content-Description", "File Transfer")
	c.Response().Header().Set("Content-Transfer-Encoding", "binary")
	c.Response().Header().Set("Expires", "0")
	c.Response().Header().Set("Cache-Control", "must-revalidate")
	c.Response().Header().Set("Pragma", "public")
	c.Response().Header().Set("Access-Control-Expose-Headers", "x-filename")

	return c.Stream(http.StatusOK, "application/octet-stream", bytes.NewReader(body))
}

func SendAudioFile(c echo.Context, filename string, body []byte) error {
	c.Response().Header().Set("Content-Type", "audio/mpeg")
	c.Response().Header().Set("Content-Disposition", "inline; filename="+url.QueryEscape(filename))
	c.Response().Header().Set("x-filename", url.QueryEscape(filename))
	c.Response().Header().Set("Content-Description", "File Transfer")
	c.Response().Header().Set("Content-Transfer-Encoding", "binary")
	c.Response().Header().Set("Expires", "0")
	c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Response().Header().Set("Pragma", "public")
	c.Response().Header().Set("Access-Control-Expose-Headers", "x-filename")

	return c.Stream(http.StatusOK, "application/octet-stream", bytes.NewReader(body))
}
