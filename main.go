package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lextoumbourou/goodhosts"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)
// Hosts map key is url, string path to config
type Hosts map[string]string

const (
	pathApacheSitesAvailable = "/etc/apache2/sites-available/"
	pathVarWWW               = "/var/www/"
	pathInitdApache2         = "/etc/init.d/apache2"
	defaultApacheConf        = "000-default.conf"
	defaultApacheSSLConf     = "default-ssl.conf"
	defaultTemplate          = "template.txt"
	defaultLocalhostIPv4     = "127.0.0.1"
)

// ListResponse struct which allow response array of hosts /api/list
type ListResponse struct {
	Hosts `json:"hosts"`
	Error string `json:"error"`
}

// HostResponse struct which allow response error or log create/delete host
type HostResponse struct {
	Data string `json:"data"`
	Error string `json:"error"`
}

// Request allow request name from Post
type Request struct {
	Name string `json:"name"`
}


func findHosts() (Hosts, error) {
	hosts := Hosts{}
	if err := filepath.Walk(
		pathApacheSitesAvailable,
		func(path string, info os.FileInfo, err error) error {
			if info.IsDir() ||
				info.Name() == defaultApacheConf ||
				info.Name() == defaultApacheSSLConf {
				return nil
			}

			url := filepath.Base(path)
			url = strings.Replace(url, ".conf", "", 1)
			hosts[url] = path
			return nil
		},
	); err != nil {
		return nil, err
	}

	return hosts, nil
}

func findConfigs() ([]string, error){
	var config []string
	hosts, err := findHosts()
	if err != nil{
		return nil, err
	}

	for _, value := range hosts{
		config = append(config, value)
	}
	return config, nil
}

// hostExists returns true if the host was already created
func hostExists(name string) (bool, error) {
	if name == "" {
		return false, errors.New("invalid host name (empty)")
	}
	path := pathApacheSitesAvailable + name + ".conf"
	configs, err := findConfigs()
	if err != nil {
		return false, err
	}
	return IndexOf(configs, path) >= 0, nil
}

// IndexOf returns either the index of the value in the slice
// or -1 if val isn't contained in the slice
func IndexOf(slice []string, val string) int {
	for i, item := range slice {
		if item == val {
			return i
		}
	}
	return -1
}
//delete host
func deleteHost(input string) (string, error) {
	textLog := ""
	name := strings.Replace(input, "delete", "", 1)
	name = strings.TrimSpace(name)
	serverPath := pathVarWWW + strings.ReplaceAll(name, ".", "")
	hosts, err := goodhosts.NewHosts()
	if err != nil {
		return "", err
	}
	fmt.Printf("Destroying host %q", name)
	out, _ := exec.Command("a2dissite", name).CombinedOutput()

	textLog = textLog + "\n" + string(out)
	if err = os.Remove(pathApacheSitesAvailable + name + ".conf"); err != nil {
		return "", err
	}
	if err = os.Remove(serverPath); err != nil {
		return "", err
	}

	if err = hosts.Remove(defaultLocalhostIPv4, name); err != nil {
		return "", err
	}
	if err := hosts.Flush(); err != nil {
		return "", err
	}
	out, err = exec.Command(pathInitdApache2, "restart").CombinedOutput()
	if err != nil {
		return "", err
	}
	textLog = textLog + "\n" + string(out)
	textLog = textLog + "\n" + "Host destroyed " + name
	return textLog, nil
}


// createHost creates a new host
func createHost(name string) (string, error){
	check, err := hostExists(name)
	if err != nil {
		return "", err
	}
	if check {
		return "", errors.New("this host already exist")
	}
	textLog := ""
	serverPath := pathVarWWW + strings.ReplaceAll(name, ".", "")
	b, err := ioutil.ReadFile(defaultTemplate)
	if err != nil {
		return "", err
	}
	template := string(b)
	template = strings.ReplaceAll(template, "{{servername}}", name)
	template = strings.ReplaceAll(template, "{{serverpath}}", serverPath)

	pathToFile := pathApacheSitesAvailable + name + ".conf"
	f, err := os.Create(pathToFile)
	if err != nil {
		return "", err
	}
	err = f.Close()
	if err != nil {
		return "", err
	}

	if err = ioutil.WriteFile(pathToFile, []byte(template), 0644); err != nil {
		return "", err
	}
	if err = os.Mkdir(serverPath, 0777); err != nil {
		return "", err
	}

	out, err := exec.Command("a2ensite", name).CombinedOutput()
	if err != nil {
		return "", err
	}

	hosts, err := goodhosts.NewHosts()
	if err != nil {
		return "", err
	}

	if !hosts.Has(defaultLocalhostIPv4, name) {
		if err = hosts.Add(defaultLocalhostIPv4, name); err != nil {
			return "", err
		}

		if err := hosts.Flush(); err != nil {
			return "", err
		}
	}

	textLog = textLog + "\n" + string(out)

	out, err = exec.Command(pathInitdApache2, "restart").CombinedOutput()
	if err != nil {
		return "", err
	}
	textLog = textLog + "\n" + string(out)

	return textLog, nil
}

func getHosts(w http.ResponseWriter, _ *http.Request) {

	hosts, err := findHosts()
	rawResponse := ListResponse{
		Hosts: hosts,
	}
	if err != nil {
		rawResponse.Error = err.Error()
	}
	resp, err := json.Marshal(rawResponse)
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp)
}

func deleteHostHandler(w http.ResponseWriter, r *http.Request){
	if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		var request Request
		err := decoder.Decode(&request)
		if err != nil {
			panic(err)
		}
		log.Println(request.Name)
		textLog, err := deleteHost(request.Name)
		rawResponse := HostResponse{}
		if err != nil{
			rawResponse.Error = err.Error()
		}else{
			rawResponse.Data = textLog
		}
		resp, err := json.Marshal(rawResponse)
		if err != nil {
			panic(err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	} else {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	}
}

func createHostHandler(w http.ResponseWriter, r *http.Request){

	if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		var request Request
		err := decoder.Decode(&request)
		if err != nil {
			panic(err)
		}
		log.Println(request.Name)
		textLog, err := createHost(request.Name)
		rawResponse := HostResponse{}
		if err != nil{
			rawResponse.Error = err.Error()
		}else{
			rawResponse.Data = textLog
		}
		resp, err := json.Marshal(rawResponse)
		if err != nil {
			panic(err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	} else {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	}
}

func main() {
	http.HandleFunc("/api/list", getHosts)
	http.HandleFunc("/api/create", createHostHandler)
	http.HandleFunc("/api/delete", deleteHostHandler)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
