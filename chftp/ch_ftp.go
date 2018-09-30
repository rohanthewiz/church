package chftp

import (
	"github.com/rohanthewiz/church/chweb/util/timeutil"
	"github.com/rohanthewiz/roftp"
	"github.com/rohanthewiz/serr"
	"github.com/jlaffaye/ftp"
	"github.com/rohanthewiz/church/chweb/config"
	"net/url"
	"path"
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
func NewCemaUploader(srcFile, destFile string) *Uploader {
	ftpOpts := roftp.FTPOptions{
		User:   config.Options.FTP.Main.User,
		Word:   config.Options.FTP.Main.Word,
		Server: config.Options.FTP.Main.Host,
		Port: config.Options.FTP.Main.Port,
	}
	if ftpOpts.Port == "" {
		ftpOpts.Port = defaultFtpPort
	}
	serverPath := serverPathPrefix + timeutil.CurrentYear() // sweet!
	return &Uploader{ ftpOpts, srcFile, serverPath, destFile }
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
		logger.LogErrAsync(err, "msg", "could not parse cema ftp domain")
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

