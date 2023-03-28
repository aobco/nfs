// Copyright © 2017 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: BSD-2-Clause
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/aobco/log"
	"github.com/aobco/nfs/nfs3"
	"github.com/aobco/nfs/nfs3/rpc"
	"io"
	"io/ioutil"
	"os"
)

func main() {
	host := "100.253.50.248"
	root := "/search01"
	mkdirName := "test_mkdir"
	log.Infof("host=%s target=%s dir=%s\n", host, root, mkdirName)
	mount, err := nfs3.DialMount(host)
	if err != nil {
		log.Fatalf("unable to dial MOUNT service: %v", err)
	}
	defer mount.Close()

	auth := rpc.NewAuthUnix("machine-name", 0, 0)
	v, err := mount.Mount(root, auth.Auth())
	if err != nil {
		log.Fatalf("unable to mount volume: %v", err)
	}
	defer v.Close()

	dirs, err := ls(v, ".")
	if err != nil {
		log.Fatalf("ls: %s", err.Error())
	}
	baseDirCount := len(dirs)

	if err := v.RemoveAll(mkdirName); err != nil {
		log.Fatalf("rmdir %v", err)
	}

	_, err = v.Mkdir(mkdirName, 0775)
	if err != nil {
		log.Fatalf("mkdir error: %v", err)
	}
	// make a nested mkdirName
	//if _, err = v.Mkdir(mkdirName+"/a/b", 0775); err != nil {
	//	log.Fatalf("mkdir error: %v", err)
	//}
	info, fh, err := v.Lookup("/")
	if err != nil {
		log.Fatalf("mkdir error: %v", err)
	}
	println(info)
	if err = v.Rename(fh, mkdirName, fh, mkdirName+"_rename"); err != nil {
		log.Fatalf("rename error: %v", err)
	}

	dirs, err = ls(v, ".")
	if err != nil {
		log.Fatalf("ls: %s", err.Error())
	}
	// check the length.  There should only be 1 entry in the target (aside from . and .., et al)
	if len(dirs) != 1+baseDirCount {
		log.Fatalf("expected %d dirs, got %d", 1+baseDirCount, len(dirs))
	}

	// 10 MB file
	if err = testFileRW(v, "10mb", 10*1024*1024); err != nil {
		log.Fatalf("fail")
	}

	// 7b file
	if err = testFileRW(v, "7b", 7); err != nil {
		log.Fatalf("fail")
	}

	// should return an error
	if err = v.RemoveAll("7b"); err == nil {
		log.Fatalf("expected a NOTADIR error")
	} else {
		nfserr := err.(*nfs3.Error)
		if nfserr.ErrorNum != nfs3.NFS3ErrNotDir {
			log.Fatalf("Wrong error")
		}
	}

	if err = v.Remove("7b"); err != nil {
		log.Fatalf("rm(7b) err: %s", err.Error())
	}

	if err = v.Remove("10mb"); err != nil {
		log.Fatalf("rm(10mb) err: %s", err.Error())
	}

	_, _, err = v.Lookup(mkdirName)
	if err != nil {
		log.Fatalf("lookup error: %s", err.Error())
	}

	if _, err = ls(v, "."); err != nil {
		log.Fatalf("ls: %s", err.Error())
	}

	if err = v.RmDir(mkdirName); err == nil {
		log.Fatalf("expected not empty error")
	}

	for _, fname := range []string{"/one", "/two", "/a/one", "/a/two", "/a/b/one", "/a/b/two"} {
		if err = testFileRW(v, mkdirName+fname, 10); err != nil {
			log.Fatalf("fail")
		}
	}

	if err = v.RemoveAll(mkdirName); err != nil {
		log.Fatalf("error removing files: %s", err.Error())
	}

	outDirs, err := ls(v, ".")
	if err != nil {
		log.Fatalf("ls: %s", err.Error())
	}

	if len(outDirs) != baseDirCount {
		log.Fatalf("directory should be empty of our created files!")
	}

	if err = mount.Unmount(); err != nil {
		log.Fatalf("unable to umount target: %v", err)
	}

	mount.Close()
	log.Infof("Completed tests")
}

func testFileRW(v *nfs3.Target, name string, filesize uint64) error {

	// create a temp file
	f, err := os.Open("/dev/urandom")
	if err != nil {
		log.Errorf("error openning random: %s", err.Error())
		return err
	}

	wr, err := v.OpenFile(name, 0777)
	if err != nil {
		log.Errorf("write fail: %s", err.Error())
		return err
	}

	// calculate the sha
	h := sha256.New()
	t := io.TeeReader(f, h)

	// Copy filesize
	n, err := io.CopyN(wr, t, int64(filesize))
	if err != nil {
		log.Errorf("error copying: n=%d, %s", n, err.Error())
		return err
	}
	expectedSum := h.Sum(nil)

	if err = wr.Close(); err != nil {
		log.Errorf("error committing: %s", err.Error())
		return err
	}

	//
	// get the file we wrote and calc the sum
	rdr, err := v.Open(name)
	if err != nil {
		log.Errorf("read error: %v", err)
		return err
	}

	h = sha256.New()
	t = io.TeeReader(rdr, h)

	_, err = ioutil.ReadAll(t)
	if err != nil {
		log.Errorf("readall error: %v", err)
		return err
	}
	actualSum := h.Sum(nil)

	if bytes.Compare(actualSum, expectedSum) != 0 {
		log.Fatalf("sums didn't match. actual=%x expected=%s", actualSum, expectedSum) //  Got=0%x expected=0%x", string(buf), testdata)
	}

	log.Infof("Sums match %x %x", actualSum, expectedSum)
	return nil
}

func ls(v *nfs3.Target, path string) ([]*nfs3.EntryPlus, error) {
	dirs, err := v.ReadDirPlus(path)
	if err != nil {
		return nil, fmt.Errorf("readdir error: %s", err.Error())
	}
	log.Infof("dirs:")
	for _, dir := range dirs {
		log.Infof("\t%s\t%d:%d\t0%o", dir.FileName, dir.Attr.Attr.UID, dir.Attr.Attr.GID, dir.Attr.Attr.Mode)
	}
	return dirs, nil
}
