package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Data struct {
	ServiceNamePill, ServiceNameTitle, ServiceNameLower, Route, Port string
}

func main() {
	fileUrl := "https://github.com/bee-travels/node-service-template/archive/master.zip"
	//defer Cleanup("download.zip", "template")

	if err := DownloadFile("download.zip", fileUrl); err != nil {
		panic(err)
	}

	err := Unzip("download.zip", "template")
	if err != nil {
		panic(err)
	}

	/*
			.ServiceNamePill = destination-basic
		.ServiceNameTitle = Destination Basic
		.ServiceNameLower = destination basic
		.Route = destinations
		.Port = 9201
	*/

	data := Data{"destination-basic", "Destination Basic", "destination basic", "destinations", "9201"}

	// files, err := OSReadDir("./template")
	// fmt.Println(files)
	ListFilesRecursive("template", data)

}

func ListFilesRecursive(folder string, data Data) error {
	err := filepath.Walk(folder,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fmt.Println(path)
			if info.Mode().IsRegular() {
				b, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}

				filecontent := string(b)
				fileTemplate := template.Must(template.New("file").Parse(filecontent))
				buf := new(bytes.Buffer)

				if err := fileTemplate.Execute(buf, data); err != nil {
					return err
				}

				if err := ioutil.WriteFile(path, buf.Bytes(), 0644); err != nil {
					return err
				}

				buf.Reset()

				filenameTemplate := template.Must(template.New("filename").Parse(info.Name()))

				filenameTemplate.Execute(buf, data)
				fileName := string(buf.Bytes())
				if fileName != info.Name() {
					newPath := strings.ReplaceAll(path, info.Name(), fileName)
					os.Rename(path, newPath)
				}
			}
			return nil
		})
	if err != nil {
		return err
	}
	return nil
}

func Cleanup(filepaths ...string) error {
	for _, filepath := range filepaths {
		fi, err := os.Stat(filepath)
		if err != nil {
			return err
		}
		mode := fi.Mode()
		if mode.IsDir() {
			if err := os.RemoveAll(filepath); err != nil {
				return err
			}
		} else if mode.IsRegular() {
			if err := os.Remove(filepath); err != nil {
				return err
			}
		}
	}
	return nil
}

func OSReadDir(root string) ([]string, error) {
	var files []string
	f, err := os.Open(root)
	if err != nil {
		return files, err
	}
	fileInfo, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return files, err
	}

	for _, file := range fileInfo {
		files = append(files, file.Name())
	}
	return files, nil
}

func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
