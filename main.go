package main

import (
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

var (
	rootDir     = os.Getenv("ROOT_DIR")
	urlBasePath = os.Getenv("URL_BASE_PATH")
	listen      = os.Getenv("LISTEN")
	debug, _    = strconv.ParseBool(os.Getenv("DEBUG"))
	logFormat   = os.Getenv("LOG_FORMAT")
)

func init() {
	if rootDir == "" {
		rootDir = "/uploads"
	}

	if urlBasePath == "" {
		urlBasePath = "/"
	}

	if listen == "" {
		listen = ":80"
	}

	if logFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else if logFormat == "logfmt" {
		log.SetFormatter(&log.TextFormatter{DisableColors: true, FullTimestamp: true})
	} else {
		log.SetFormatter(&log.TextFormatter{ForceColors: true})
	}

	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

func main() {
	log.Info("Staring simple upload server.")

	http.HandleFunc(urlBasePath, handleRoot)
	log.Panic(http.ListenAndServe(listen, nil))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	log.Debugf("%s: %s", r.Method, r.URL.Path)

	if r.Method == http.MethodGet {
		http.StripPrefix(urlBasePath, http.FileServer(http.Dir(rootDir))).ServeHTTP(w, r)

		return
	} else if r.URL.Path == urlBasePath {
		log.Debugf("Unsupported method")
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)

		return
	}

	switch r.Method {
	case http.MethodPut:
		handlePut(w, r)
	case http.MethodDelete:
		handleDelete(w, r)
	default:
		log.Debug("Unsupported method")
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
	}
}

func handlePut(w http.ResponseWriter, r *http.Request) {
	filePath := urlToPath(r.URL.Path)

	err := os.MkdirAll(path.Dir(filePath), 0700)
	if err != nil {
		log.Warnf("Error while creating directory for file '%s': %s'", filePath, err)
		http.Error(w,
			"Path could not be created. Make sure the path is correct",
			http.StatusInternalServerError)

		return
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		log.Errorf("Error while opening file '%s': %s", filePath, err)
		http.Error(w, "File could not be created/opened", http.StatusInternalServerError)

		return
	}
	defer file.Close()

	var fileReader io.Reader

	formFile, _, err := r.FormFile("file")
	if err != nil {
		defer r.Body.Close() // I don't know if this is needed
		fileReader = r.Body
	} else {
		defer formFile.Close()
		fileReader = formFile
	}

	_, err = io.Copy(file, fileReader)
	if err != nil {
		log.Errorf("Error while writing file '%s': %s", filePath, err)
		http.Error(w, "File could not be saved", http.StatusInternalServerError)

		return
	}

	log.Debugf("File '%s' written", filePath)

	w.WriteHeader(http.StatusNoContent)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	subjectPath := urlToPath(r.URL.Path)

	_, err := os.Stat(subjectPath)
	if os.IsNotExist(err) {
		log.Debugf("File '%s' not found", subjectPath)
		http.NotFound(w, r)

		return
	}
	if err != nil {
		log.Errorf("Error with file stats '%s': %s", subjectPath, err)
		http.Error(w, "File could not be deleted", http.StatusInternalServerError)

		return
	}

	err = os.RemoveAll(subjectPath)
	if err != nil {
		log.Errorf("Error while removing file '%s': %s", subjectPath, err)
		http.Error(w, "File could not be deleted", http.StatusInternalServerError)

		return
	}

	log.Debugf("File '%s' deleted", subjectPath)

	w.WriteHeader(http.StatusNoContent)
}

func urlToPath(urlPath string) string {
	return path.Join(rootDir, strings.TrimPrefix(urlPath, urlBasePath))
}
