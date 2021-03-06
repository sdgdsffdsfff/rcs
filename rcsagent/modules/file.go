package modules

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (seb File_push_req) Handle(res *Atomicresponse) error {
	//download file from remote,and check the md5sum
	if err := Downloadfilefromurl(seb.Sfileurl, seb.Sfilemd5, seb.DstPath); err != nil {
		res.Flag = false
		res.Result = err.Error()
		return err
	}
	res.Flag = true
	res.Result = seb.Sfilemd5
	return nil
}
func (seb File_pull_req) Handle(res *Atomicresponse) error {
	//upload file to the specified remote
	return nil
}
func (seb File_cp_req) Handle(res *Atomicresponse) error {
	/*
		copy file or directory from source to destination , overwrite
		1.if source is a single file,just copy to the named destination file , overwrite
		2.if source is a directory,recursive copy the all files with the directory or not,determined by the 'Wodir' filed
		3.if source is a directory,and non file bellows there,Handle do nothing and return nil
	*/
	withoutdir := seb.Wodir
	sfilepath := seb.Sfilepath
	dfilepath := seb.Dfilepath
	ex, dr, err := Isexistdir(sfilepath)
	if !ex {
		res.Flag = false
		res.Result = err.Error()
		return err
	}
	if !dr {

		err := cpfile(sfilepath, dfilepath)
		if err != nil {
			res.Flag = false
			res.Result = err.Error()
			return err
		}
		res.Flag = true
		res.Result = "success!"
	} else { //is a directory ,recursive

		wf := func(path string, f os.FileInfo, err error) error {
			if f == nil {
				return err
			}
			if f.IsDir() {
				if withoutdir {
					return os.MkdirAll(filepath.Join(dfilepath, strings.TrimPrefix(path, sfilepath)), 0666)
				} else {
					return os.MkdirAll(filepath.Join(dfilepath, strings.TrimPrefix(path, filepath.Clean(sfilepath+`/../`))), 0666)
				}
			}
			if withoutdir { //just copy the sfilepath`s bellows
				return cpfile(path, filepath.Join(dfilepath, strings.TrimPrefix(path, sfilepath)))
			} else { //copy the directory 'sfilepath' and it`s bellows
				return cpfile(path, filepath.Join(dfilepath, strings.TrimPrefix(path, filepath.Clean(sfilepath+`/../`))))
			}
		}
		if err := filepath.Walk(sfilepath, wf); err != nil {
			res.Flag = false
			res.Result = err.Error()
			return err
		}
		res.Flag = true
		res.Result = "success!"
	}
	return nil
}
func (seb File_del_req) Handle(res *Atomicresponse) error {
	/*
		delete the specified file or directory,depends on  the 'Wobak' filed,do backup or not,
		'backup and delete' is just call 'os.rename' function
	*/
	if seb.Wobak { //without bak
		err := os.RemoveAll(seb.Sfilepath)
		if err != nil {
			res.Flag = false
			res.Result = err.Error()
			return err
		}
		res.Flag = true
		res.Result = "success!"
	} else { //with bak
		t := time.Now().Unix()
		dfilepath := seb.Sfilepath + `-bk` + strconv.FormatInt(t, 10)
		err := os.Rename(seb.Sfilepath, dfilepath) //call os.rename for backup and delete
		if err != nil {
			res.Flag = false
			res.Result = err.Error()
			return err
		}
		res.Flag = true
		res.Result = "success,backup in " + dfilepath

	}
	return nil
}
func (seb File_rename_req) Handle(res *Atomicresponse) error {
	dfilepath := filepath.Join(filepath.Dir(filepath.Clean(seb.Sfilepath)), seb.Newname)
	err := os.Rename(seb.Sfilepath, dfilepath) //call os.rename for backup and delete
	if err != nil {
		res.Flag = false
		res.Result = err.Error()
		return err
	}
	res.Flag = true
	res.Result = "success"
	return nil
}
func (seb File_grep_req) Handle(res *Atomicresponse) error {
	//like linux 'grep' command
	fd, err := os.Open(seb.Sfilepath)
	if err != nil {
		res.Flag = false
		res.Result = err.Error()
		return err
	}
	defer fd.Close()
	//rx := regexp.MustCompile(seb.Patternstr)
	bufrd := bufio.NewReader(fd)
	var linestr string
	var rs bool
	for err != io.EOF {
		linestr, err = bufrd.ReadString('\n')
		if rs, _ = regexp.MatchString(seb.Patternstr, linestr); rs {
			res.Result += linestr
		}
	}
	res.Flag = true
	return nil
}
func (seb File_replace_req) Handle(res *Atomicresponse) error {
	//replace the Patternstr of specified file to relptext,like sed -i s/Patternstr/relptext/g file
	fi, err := os.Stat(seb.Sfilepath)
	if err != nil {
		res.Flag = false
		res.Result = err.Error()
		return err
	}
	content, err := ioutil.ReadFile(seb.Sfilepath) //ioutil.ReadFile read the hole content to  memory once,that`s a risk point for a 'huge file'
	if err != nil {
		res.Flag = false
		res.Result = err.Error()
		return err
	}
	rx := regexp.MustCompile(seb.Patternstr)
	if !rx.Match(content) || seb.Repltext == seb.Patternstr {
		res.Flag = true
		res.Result = seb.Sfilepath + `  ` + "Nochanged\n"
		return nil
	}
	//content = rx.ReplaceAll(content, []byte(seb.Repltext))
	content = rx.ReplaceAllLiteral(content, []byte(seb.Repltext))
	if err := ioutil.WriteFile(seb.Sfilepath, content, fi.Mode()); err != nil {
		res.Flag = false
		res.Result = err.Error()
		return err
	}
	res.Flag = true
	res.Result = seb.Sfilepath + `  ` + "Changed\n"
	return nil
}
func (seb File_mreplace_req) Handle(res *Atomicresponse) error {
	/*replace the Patternstr of the succesive match files in a directory to relptext,this means that:
	1.find the match files in a directory
	2.replace there files
	*/
	err, files := Listmatchfiles(seb.Sfiledir, seb.Filenamepatternstr)
	if err != nil {
		res.Flag = false
		res.Result = err.Error()
		return err
	}
	req := new(File_replace_req)
	req.Patternstr = seb.Patternstr
	req.Repltext = seb.Repltext
	eachres := new(Atomicresponse)

	if len(files) == 0 {
		res.Flag = true
		res.Result = "No matched files"
	}

	for _, file := range files {
		req.Sfilepath = file
		if err := req.Handle(eachres); err != nil { //may partly return
			return err
		}
		res.Result += eachres.Result
	}
	res.Flag = true
	return nil
}
func (seb File_md5sum_req) Handle(res *Atomicresponse) error {
	//compute the md5sum of the specified file ,or all files in a directory
	// output format : RWOSFR2FFSDFADF898DF:::/tmp/test/sdf.ini
	ex, dr, err := Isexistdir(seb.Sfilepath)
	if !ex {
		res.Flag = false
		res.Result = err.Error()
		return err
	}
	if !dr {
		md5s, _ := FileMd5(seb.Sfilepath)
		res.Flag = true
		res.Result = md5s + `:::` + seb.Sfilepath
	} else {
		wf := func(path string, f os.FileInfo, err error) error {
			if f == nil {
				return err
			}
			if f.IsDir() {
				return nil
			}
			md5s, err := FileMd5(path)
			if err != nil {
				return err
			}
			res.Flag = true
			res.Result += md5s + `:::` + path + "\n"
			return nil
		}
		if err := filepath.Walk(seb.Sfilepath, wf); err != nil {
			res.Flag = false
			res.Result = err.Error()
			return err
		}
	}
	return nil
}
func (seb File_ckmd5sum_req) Handle(res *Atomicresponse) error {
	//check the md5sum according to a md5file,like md5sum -c file
	/* the md5file format :
	RWOSFR2FFSDFADF898DF:::/tmp/test/sdf.ini
	RWOSFR2FFSDFADF898DF:::/tmp/test/set.sh
	*/
	fd, err := os.Open(seb.Md5filepath)
	if err != nil {
		res.Flag = false
		res.Result = err.Error()
		return err
	}
	defer fd.Close()
	bufrd := bufio.NewReader(fd)
	entry := make([]string, 2)
	var linestr, md5s string
	var RIGHT, WRONG, ERR int

	for err != io.EOF {
		linestr, err = bufrd.ReadString('\n')
		//windows file line end with '\r\n';unix-like file line end with '\n',so should trim '\n' and '\r' by step
		linestr = strings.TrimSuffix(linestr, "\n")
		linestr = strings.TrimSuffix(linestr, "\r")
		entry = strings.Split(linestr, `:::`)
		if len(entry) != 2 { //filter black line and wrong format line
			continue
		}
		md5s, err = FileMd5(entry[1])
		if err == nil {
			if md5s == entry[0] {
				res.Result += entry[1] + `:::CHECK RIGHT` + "\n"
				RIGHT++
			} else {
				res.Result += entry[1] + `:::CHECK WRONG` + "\n"
				WRONG++
			}
		} else {
			res.Result += entry[1] + `:::` + err.Error() + "\n"
			ERR++
		}
	}
	res.Flag = true
	res.Result += fmt.Sprintf("------Statistics,RIGHT:%d,WRONG:%d,ERROR:%d------", RIGHT, WRONG, ERR)
	return nil
}
func (seb File_zip_req) Handle(res *Atomicresponse) error {
	//will not auto add '.zip' subfix to Zipfilepath
	if seb.Zipfilepath == "" {
		seb.Zipfilepath = filepath.Join(filepath.Dir(filepath.Clean(seb.Sfilepath)), strings.TrimSuffix(filepath.Base(seb.Sfilepath), filepath.Ext(seb.Sfilepath))) + `.zip`
	}
	zipfile, err := os.Create(seb.Zipfilepath)
	if err != nil {
		res.Result = err.Error()
		return err
	}
	defer zipfile.Close()
	zipfilewr := bufio.NewWriterSize(zipfile, 10*1024*1024)
	zipwriter := zip.NewWriter(zipfilewr)
	br := bufio.NewReader(bytes.NewReader(make([]byte, 10*1024*1024)))
	wf := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name, err = filepath.Rel(filepath.Dir(seb.Sfilepath), path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			header.Method = 8
			fd, err := os.Open(path)
			if err != nil {
				return err
			}
			br.Reset(fd)
		}
		w, err := zipwriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = br.WriteTo(w)
		if err != nil {
			return err
		}
		return nil
	}
	err = filepath.Walk(seb.Sfilepath, wf)
	if err != nil {
		res.Result = err.Error()
		return err
	}
	if err := zipfilewr.Flush(); err != nil {
		res.Result = err.Error()
		return err
	}
	if err := zipwriter.Close(); err != nil {
		res.Result = err.Error()
		return err
	}
	res.Flag = true
	res.Result = "success"
	return nil
}
func (seb File_unzip_req) Handle(res *Atomicresponse) error {
	if seb.Dstdir == "" {
		seb.Dstdir = filepath.Dir(seb.Zipfilepath)
	}
	var dest string
	if seb.Wdir {
		dest = filepath.Join(seb.Dstdir, strings.TrimSuffix(filepath.Base(seb.Zipfilepath), filepath.Ext(seb.Zipfilepath)))
	} else {
		dest = seb.Dstdir
	}

	unzip_file, err := zip.OpenReader(seb.Zipfilepath)
	if err != nil {
		res.Result = err.Error()
		return err
	}
	os.MkdirAll(dest, 0755)
	for _, f := range unzip_file.File {
		rc, err := f.Open()
		if err != nil {
			res.Result = err.Error()
			return err
		}
		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				res.Result = err.Error()
				return err
			}
			_, err = io.Copy(f, rc)
			if err != nil {
				if err != io.EOF {
					res.Result = err.Error()
					return err
				}
			}
			f.Close()
		}
	}
	unzip_file.Close()
	res.Flag = true
	res.Result = "success"
	return nil
}
