package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/mitchellh/go-homedir"
)

type Data struct {
	ServiceNamePill, ServiceNameTitle, ServiceNameLower, Route, Port string
}

var Usage string

func init() {
	Usage = "Usage:\n  bee-bootstrap [<github-url> | node | python | go]"
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println(Usage)
		os.Exit(0)
	}

	baseURL := os.Args[1]
	var fileURL string
	if strings.HasPrefix(baseURL, "https://") || strings.HasPrefix(baseURL, "http://") {
		fileURL = baseURL + "/archive/master.zip"
	} else if strings.HasPrefix(baseURL, "github.com/") {
		fileURL = "https://" + baseURL + "/archive/master.zip"
	} else {
		fileURL = "https://github.com/bee-travels/" + baseURL + "-service-template/archive/master.zip"
	}

	/*Service name (destination-basic):
	Service route (destinations):
	Service port (9201): */
	var serviceName string
	var route string
	var port string

	fmt.Print("Service name (destination-basic): ")
	fmt.Scan(&serviceName)

	fmt.Print("Service route (destinations): ")
	fmt.Scan(&route)

	fmt.Print("Service port (9000): ")
	fmt.Scan(&port)

	data, err := GetData(serviceName, route, port)
	if err != nil {
		fmt.Println("error : ", err)
		os.Exit(1)
	}

	homeDir, err := homedir.Dir()
	if err != nil {
		fmt.Println("error getting home directory ", err)
		os.Exit(1)
	}

	fmt.Println(homeDir)
	path := homeDir + "/.bee-bootstrap"
	CheckFolder(path)

	defer Cleanup(path+"/download.zip", path+"/template")

	if err := DownloadFile(path+"/download.zip", fileURL); err != nil {
		fmt.Println("could not download file ", err)
		os.Exit(1)
	}

	err = Unzip(path+"/download.zip", path+"/template")
	if err != nil {
		fmt.Println("could not unzip file ", err)
		os.Exit(1)
	}

	err = ProcessFiles(path+"/template", *data)
	if err != nil {
		fmt.Println("could not process files ", err)
	}

	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(pwd)
	err = MoveFile(path+"/template", data.ServiceNamePill)
	if err != nil {
		fmt.Println("could not move file", err)
		os.Exit(1)
	}

	fmt.Println("Done!")
}

func MoveFile(root, dst string) error {
	// os.Rename("/Users/mofizur.rahman@ibm.com/.bee-bootstrap/template/node-service-template-master",
	// 	"./destination-service")
	files, err := OSReadDir(root)
	if err != nil {
		return err
	}

	if len(files) != 1 {
		return fmt.Errorf("expected only one file")
	}

	src := root + "/" + files[0]

	err = os.Rename(src, dst)
	if err != nil {
		return err
	}
	return nil
}

func GetData(serviceNamePill, route, port string) (*Data, error) {
	if serviceNamePill == "" || route == "" || port == "" {
		return nil, fmt.Errorf("no data provided")
	}

	if strings.Contains(serviceNamePill, " ") {
		return nil, fmt.Errorf("service name should not contain spaces")
	}

	if strings.Contains(serviceNamePill, "_") {
		return nil, fmt.Errorf("do not use _ in service names")
	}

	route = strings.ToLower(route)

	if _, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("port should be a number")
	}

	serviceNamePill = strings.ToLower(serviceNamePill)
	serviceNameLower := strings.ReplaceAll(serviceNamePill, "-", " ")
	serviceNameTitle := strings.Title(serviceNameLower)

	return &Data{
		ServiceNamePill:  serviceNamePill,
		ServiceNameLower: serviceNameLower,
		ServiceNameTitle: serviceNameTitle,
		Route:            route,
		Port:             port,
	}, nil

}

// OSReadDir returns the list of directory/file in a folder
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

// CheckFolder if the folder does not exist create it
func CheckFolder(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Println("creating folder : ", path)
		os.Mkdir(path, 0777)
	}
}

// ProcessFiles with the template data
func ProcessFiles(folder string, data Data) error {
	err := filepath.Walk(folder,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.Mode().IsRegular() {
				fmt.Println(path)
				b, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}

				filecontent := string(b)
				fileTemplate, err := template.New("file").Parse(filecontent)

				if err != nil {
					return nil
				}

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

// Cleanup folder
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

// Unzip from src to destination
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
