package chftp

import (
	"net/url"
	"path"
	"strings"

	"github.com/jlaffaye/ftp"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/roftp"
	"github.com/rohanthewiz/serr"

	"github.com/rohanthewiz/logger"
)

const (
	defaultFtpPort   = "21"
	serverPathPrefix = "/sermons/"
)

type Uploader struct {
	opts         roftp.FTPOptions
	srcPath      string
	serverPath   string
	destFilename string
}

// Todo: tests!!

// Create New Uploader object with derived options
// No connection is made at this point
func NewCemaUploader(srcFile, destFile, dateTaught string) *Uploader {
	ftpOpts := roftp.FTPOptions{
		User:   config.Options.FTP.Main.User,
		Word:   config.Options.FTP.Main.Word,
		Server: config.Options.FTP.Main.Host,
		Port:   config.Options.FTP.Main.Port,
	}
	if ftpOpts.Port == "" {
		ftpOpts.Port = defaultFtpPort
	}

	year := timeutil.CurrentYear()
	if arr := strings.SplitN(dateTaught, "-", 2); len(arr) == 2 {
		year = arr[0]
	}

	return &Uploader{ftpOpts, srcFile, serverPathPrefix + year, destFile}
}

// Do all the things
func (u *Uploader) Run() error {
	conn, err := roftp.NewFTPConn(u.opts)
	if err != nil {
		return serr.Wrap(err, "msg", "Unable to login", "Port", u.opts.Port)
	}
	defer conn.Quit()

	println("Uploading", u.srcPath, "to server", u.opts.Server)
	err = roftp.UploadFile(conn, u.srcPath, u.serverPath, u.destFilename)
	if err != nil {
		return serr.Wrap(err, "msg", "Error uploading file", "serverPath", u.serverPath)
	}

	err = u.listAndPrintFiles(conn)
	if err != nil {
		return serr.Wrap(err, "msg", "Unable to list files on server", "serverPath", u.serverPath)
	}

	return nil
}

func (u Uploader) DestWebPath() string {
	url_, err := url.Parse(config.Options.FTP.Main.WebAccessPath)
	if err != nil {
		logger.LogErr(serr.Wrap(err, "msg", "could not parse cema ftp domain"))
		return ""
	}
	url_.Path = path.Join(url_.Path, u.serverPath, u.destFilename)
	return url_.String()
}

// Print Directory listing
func (u Uploader) listAndPrintFiles(conn *ftp.ServerConn) error {
	filesData, err := roftp.ListFiles(conn, u.serverPath)
	if err != nil {
		return serr.Wrap(err, "Unable to list files")
	}
	for _, fileData := range filesData {
		println("Name:", fileData.Name, " Size:", fileData.Size)
	}
	return nil
}
