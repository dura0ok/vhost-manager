package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/lextoumbourou/goodhosts"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func findHosts() ([]string, error) {
	var files []string
	root := "/etc/apache2/sites-available/"
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || info.Name() == "000-default.conf" || info.Name() == "default-ssl.conf" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func hostExists(name string) (bool, error) {
	checkHost := "/etc/apache2/sites-available/" + name + ".conf"
	configs, err := findHosts()
	if err != nil {
		return false, err
	}
	_, found := Find(configs, checkHost)
	return found, nil
}

func Find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func createHost(name string) error {
	check, err := hostExists(name)
	if err != nil {
		return err
	}
	if check {
		return errors.New("this host already exist")
	}
	serverPath := "/var/www/" + strings.ReplaceAll(name, ".", "")
	b, err := ioutil.ReadFile("template.txt")
	if err != nil {
		return err
	}
	var template = string(b)
	template = strings.ReplaceAll(template, "{{servername}}", name)
	template = strings.ReplaceAll(template, "{{serverpath}}", serverPath)

	pathToFile := "/etc/apache2/sites-available/" + name + ".conf"
	f, err := os.Create(pathToFile)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(pathToFile, []byte(template), 0644)
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	err = os.Mkdir(serverPath, 0777)
	if err != nil {
		return err
	}

	out, err := exec.Command("a2ensite", name).CombinedOutput()
	if err != nil {
		return err
	} else {
		/*
			f, err = os.OpenFile("/etc/hosts",
				os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}

			if _, err := f.WriteString("127.0.0.1       " + name); err != nil {
				log.Println(err)
			}

			err = f.Close()
			if err != nil {
				return err
			}
		*/
		hosts, _ := goodhosts.NewHosts()
		if !hosts.Has("127.0.0.1", name) {
			err = hosts.Add("127.0.0.1", name)
			if err != nil {
				return err
			}

			if err := hosts.Flush(); err != nil {
				return err
			}
		}
	}
	fmt.Printf("%s", out)

	out, err = exec.Command("/etc/init.d/apache2", "restart").CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s", out)

	return nil
}

func main() {
	var input string

	fmt.Println("If you want to create, virtual host write create and url\nIf you want to delete virtual host write delete and url\nIf you want to list virtual host write list")
	in := bufio.NewReader(os.Stdin)
	input, _ = in.ReadString('\n')
	input = strings.TrimSpace(input)

	switch {
	case input == "list":
		arr, err := findHosts()
		if err != nil {
			panic(err)
		}
		for _, value := range arr {
			url := filepath.Base(value)
			url = strings.Replace(url, ".conf", "", 1)
			fmt.Println("Host: http://" + url + ", config file => " + value)
		}

	case strings.Contains(input, "create"):
		name := strings.Replace(input, "create", "", 1)
		name = strings.TrimSpace(name)

		err := createHost(name)
		if err != nil {
			panic(err)
		}

	case strings.Contains(input, "delete"):
		name := strings.Replace(input, "delete", "", 1)
		name = strings.TrimSpace(name)
		serverPath := "/var/www/" + strings.ReplaceAll(name, ".", "")
		hosts, err := goodhosts.NewHosts()
		if err != nil {
			panic(err)
		}
		fmt.Println("Destroy host... " + name)
		out, err := exec.Command("a2dissite", name).CombinedOutput()
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s", out)
		err = os.Remove("/etc/apache2/sites-available/" + name + ".conf")
		//exec.Command("rm", "/etc/apache2/sites-available" + name + ".conf").CombinedOutput()
		if err != nil {
			fmt.Println("rm conf")
			panic(err)
		}
		err = os.Remove(serverPath)
		if err != nil {
			fmt.Println("rm folder")
			panic(err)
		}

		err = hosts.Remove("127.0.0.1", name)
		if err != nil {
			panic(err)
		}
		if err := hosts.Flush(); err != nil {
			panic(err)
		}
		out, err = exec.Command("/etc/init.d/apache2", "restart").CombinedOutput()
		if err != nil {
			fmt.Println("apache :(")
			panic(err)
		}
		fmt.Printf("%s", out)
		fmt.Println("Host is destroyed")
	}
}
