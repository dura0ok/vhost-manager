package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lextoumbourou/goodhosts"
)

const (
	pathApacheSitesAvailable = "/etc/apache2/sites-available/"
	pathVarWWW               = "/var/www/"
	pathInitdApache2         = "/etc/init.d/apache2"
	defaultApacheConf        = "000-default.conf"
	defaultApacheSSLConf     = "default-ssl.conf"
	defaultTemplate          = "template.txt"
	defaultLocalhostIPv4     = "127.0.0.1"
)

// findHosts returns the paths to virtual hosts configs
func findHosts() ([]string, error) {
	var files []string
	if err := filepath.Walk(
		pathApacheSitesAvailable,
		func(path string, info os.FileInfo, err error) error {
			if info.IsDir() ||
				info.Name() == defaultApacheConf ||
				info.Name() == defaultApacheSSLConf {
				return nil
			}
			files = append(files, path)
			return nil
		},
	); err != nil {
		return nil, err
	}

	return files, nil
}

// hostExists returns true if the host was already created
func hostExists(name string) (bool, error) {
	if name == "" {
		return false, errors.New("invalid host name (empty)")
	}
	path := pathApacheSitesAvailable + name + ".conf"
	configs, err := findHosts()
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

// createHost creates a new host
func createHost(name string) error {
	check, err := hostExists(name)
	if err != nil {
		return err
	}
	if check {
		return fmt.Errorf("host %q already exists", name)
	}
	serverPath := pathVarWWW + strings.ReplaceAll(name, ".", "")
	b, err := ioutil.ReadFile(defaultTemplate)
	if err != nil {
		return err
	}
	template := string(b)
	template = strings.ReplaceAll(template, "{{servername}}", name)
	template = strings.ReplaceAll(template, "{{serverpath}}", serverPath)

	pathToFile := pathApacheSitesAvailable + name + ".conf"
	f, err := os.Create(pathToFile)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = ioutil.WriteFile(pathToFile, []byte(template), 0644); err != nil {
		return err
	}
	if err = os.Mkdir(serverPath, 0777); err != nil {
		return err
	}

	out, err := exec.Command("a2ensite", name).CombinedOutput()
	if err != nil {
		return err
	}

	hosts, err := goodhosts.NewHosts()
	if err != nil {
		return err
	}

	if !hosts.Has(defaultLocalhostIPv4, name) {
		if err = hosts.Add(defaultLocalhostIPv4, name); err != nil {
			return err
		}

		if err := hosts.Flush(); err != nil {
			return err
		}
	}

	fmt.Printf("%s", out)

	out, err = exec.Command(pathInitdApache2, "restart").CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s", out)

	return nil
}

func hostCreate(input string) {
	name := strings.Replace(input, "create", "", 1)
	name = strings.TrimSpace(name)

	err := createHost(name)
	if err != nil {
		log.Fatal(err)
	}
}

func hostDelete(input string) {
	name := strings.Replace(input, "delete", "", 1)
	name = strings.TrimSpace(name)
	serverPath := pathVarWWW + strings.ReplaceAll(name, ".", "")
	hosts, err := goodhosts.NewHosts()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Destroying host %q", name)
	out, err := exec.Command("a2dissite", name).CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s", out)
	if err = os.Remove(pathApacheSitesAvailable + name + ".conf"); err != nil {
		log.Fatal("rm conf:", err)
	}
	if err = os.Remove(serverPath); err != nil {
		log.Fatal("rm folder:", err)
	}

	if err = hosts.Remove(defaultLocalhostIPv4, name); err != nil {
		log.Fatal(err)
	}
	if err := hosts.Flush(); err != nil {
		log.Fatal(err)
	}
	out, err = exec.Command(pathInitdApache2, "restart").CombinedOutput()
	if err != nil {
		log.Fatal("apache: ", err)
	}
	fmt.Printf("%s", out)
	fmt.Printf("Host %q destroyed\n", name)
}

func hostList() {
	arr, err := findHosts()
	if err != nil {
		log.Fatal(err)
	}
	for _, value := range arr {
		url := filepath.Base(value)
		url = strings.Replace(url, ".conf", "", 1)
		fmt.Println("Host: http://" + url + ", config file => " + value)
	}
}

func main() {
	fmt.Println("If you want to create, virtual host write create and url\nIf you want to delete virtual host write delete and url\nIf you want to list virtual host write list")
	in := bufio.NewReader(os.Stdin)
	input, err := in.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	input = strings.TrimSpace(input)

	switch {
	case input == "list":
		hostList()
	case strings.Contains(input, "create"):
		hostCreate(input)
	case strings.Contains(input, "delete"):
		hostDelete(input)
	}
}
