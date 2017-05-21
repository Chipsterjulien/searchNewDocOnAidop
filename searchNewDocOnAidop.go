package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/jordan-wright/email"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

type infoStruct struct {
	FileType  string
	FilesList []fileInfo
}

type fileInfo struct {
	Filename string
	FileSize int
}

func main() {
	confPath := "/etc/searchNewDocOnAidop/"
	confFilename := "searchNewDocOnAidop"
	logFilename := "/var/log/searchNewDocOnAidop/error.log"

	// confPath := "cfg/"
	// confFilename := "searchNewDocOnAidop"
	// logFilename := "error.log"

	fd := initLogging(&logFilename)
	defer fd.Close()

	loadConfig(&confPath, &confFilename)
	dataOnJSON := loadJSON()
	client := identifyToWebSite()
	webPage := getFilesInfoOnWebPage(client)
	dataOnPage := treatHTMLPage(webPage)
	// if there is file(s) in table web page and struct is different than old save data
	if len((*dataOnPage).FilesList) != 0 && !isSameData(dataOnJSON, dataOnPage) {
		storeJSON(dataOnPage)
		newFilesList := getFilesList(dataOnPage)
		tmpStr := strings.Replace(strings.Title(strings.ToLower((*dataOnPage).FileType)), "_", " ", -1)
		sendAnEmail(fmt.Sprintf("Il y a des nouveaux fichiers sur aidop:\nType: %v\n%v", tmpStr, strings.Join(*newFilesList, "\n")))
	}
}

func getFilesList(dataOnPage *infoStruct) *[]string {
	newFilesList := make([]string, len(dataOnPage.FilesList))
	for num, fi := range dataOnPage.FilesList {
		newFilesList[num] = fi.Filename
	}

	return &newFilesList
}

func getFilesInfoOnWebPage(client *http.Client) *[]byte {
	log := logging.MustGetLogger("log")

	resp, err := client.Get(viper.GetString("url.filesLocation"))
	if err != nil {
		log.Criticalf("Unable to access to final web page: %v", err)
		sendAnEmail(fmt.Sprintf("Je n'arrive pas à me connecter sur le site d'aidop: %v", err))
		os.Exit(1)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Criticalf("Unable to get final web page: %v", err)
		sendAnEmail(fmt.Sprintf("Je n'arrive pas à récupérer la page qui contient la liste des fichiers disponibles: %v", err))
		os.Exit(1)
	}
	resp.Body.Close()

	return &body
}

func identifyToWebSite() *http.Client {
	log := logging.MustGetLogger("log")

	data := url.Values{}
	data.Add("login", viper.GetString("default.login"))
	data.Add("password", viper.GetString("default.password"))

	jar, _ := cookiejar.New(nil)

	client := &http.Client{
		Jar: jar,
	}

	// resp, err := client.PostForm(viper.GetString("url.webSite"), data)
	_, err := client.PostForm(viper.GetString("url.webSite"), data)
	if err != nil {
		log.Criticalf("Unable to login on web site: %v", err)
		sendAnEmail(fmt.Sprintf("Unable to login on web site: %v", err))
		os.Exit(1)
	}
	// defer resp.Body.Close()
	// resp.Body.Close()

	return client
}

func isSameData(dataOnJSON, dataOnPage *infoStruct) bool {
	if (*dataOnJSON).FileType != (*dataOnPage).FileType {
		return false
	} else if len((*dataOnJSON).FilesList) != len((*dataOnPage).FilesList) {
		return false
	}
	for num, val := range (*dataOnJSON).FilesList {
		if val.Filename != (*dataOnPage).FilesList[num].Filename {
			return false
		}
		if val.FileSize != (*dataOnPage).FilesList[num].FileSize {
			return false
		}
	}

	return true
}

func loadJSON() *infoStruct {
	log := logging.MustGetLogger("log")
	var data infoStruct

	byteFile, err := ioutil.ReadFile(viper.GetString("default.savedJSONFile"))
	if err != nil {
		return &data
	}

	if err := json.Unmarshal(byteFile, &data); err != nil {
		// Reset data info before return info
		data = infoStruct{}
		log.Warningf("Unable to unmarshal JSON: %v", err)
	}

	return &data
}

func sendAnEmail(message string) {
	log := logging.MustGetLogger("log")

	host := viper.GetString("email.smtp")
	hostNPort := fmt.Sprintf("%s:%s", host, viper.GetString("email.port"))
	username := viper.GetString("email.login")
	password := viper.GetString("email.password")
	to := viper.GetStringSlice("email.sendTo")

	e := email.NewEmail()
	e.From = viper.GetString("email.from")
	e.To = to
	e.Subject = "searchNewDocOnAidop à trouver de nouveaux fichiers sur Aidop"
	e.Text = []byte(message)
	if err := e.Send(hostNPort, smtp.PlainAuth("", username, password, host)); err != nil {
		log.Warningf("Unable to send an email to \"%s\": %v", strings.Join(to, " "), err)
	} else {
		log.Debugf("Email was sent to \"%s\"", strings.Join(to, " "))
	}
}

func storeJSON(dataOnPage *infoStruct) {
	log := logging.MustGetLogger("log")

	body, err := json.Marshal(dataOnPage)
	if err != nil {
		log.Criticalf("Unable to convert struct to JSON: %v", err)
		sendAnEmail(fmt.Sprintf("Impossible de convertir les données de la page en JSON: %v", err))
		os.Exit(1)
	}

	if err := ioutil.WriteFile(viper.GetString("default.savedJSONFile"), body, 0744); err != nil {
		log.Criticalf("Unable to write JSON file: %v", err)
		sendAnEmail(fmt.Sprintf("Impossible de sauvegarder le fichier JSON: %v", err))
		os.Exit(1)
	}
}

func treatHTMLPage(page *[]byte) *infoStruct {
	log := logging.MustGetLogger("log")
	info := infoStruct{}

	pageSplitted := bytes.Split(*page, []byte("\n"))
	for num, line := range pageSplitted {
		if bytes.Contains(line, []byte("value=\"64\"")) {
			fileType := bytes.Replace(line, []byte("<option value=\"64\" >"), []byte(""), -1)
			fileType = bytes.Replace(fileType, []byte("</option>"), []byte(""), -1)
			fileType = bytes.TrimSpace(fileType)

			info.FileType = string(fileType)
		} else if bytes.Contains(line, []byte("name=\"fichier\"")) && (num+2) < len(pageSplitted) {
			f := fileInfo{}
			var err error

			filename := bytes.Replace(line, []byte("<input name=\"fichier\" type=\"hidden\" value=\""), []byte(""), -1)
			filename = bytes.Replace(filename, []byte("\"/>"), []byte(""), -1)
			f.Filename = string(bytes.TrimSpace(filename))

			fileSizeByte := pageSplitted[num+2]
			fileSizeByte = bytes.Replace(fileSizeByte, []byte("<input name=\"taille\" type=\"hidden\" value=\""), []byte(""), -1)
			fileSizeByte = bytes.Replace(fileSizeByte, []byte("\"/>"), []byte(""), -1)
			fileSizeByte = bytes.TrimSpace(fileSizeByte)
			f.FileSize, err = strconv.Atoi(string(fileSizeByte))
			if err != nil {
				f.FileSize = 0
				log.Warningf("Unable to convert %s into an int: %v", string(fileSizeByte), err)
			}

			info.FilesList = append(info.FilesList, f)
		}
	}

	return &info
}
