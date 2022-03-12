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
	rootDir          = os.Getenv("ROOT_DIR")
	urlBasePath      = os.Getenv("URL_BASE_PATH")
	listen           = os.Getenv("LISTEN")
	debug, _         = strconv.ParseBool(os.Getenv("DEBUG"))
	logFormat        = os.Getenv("LOG_FORMAT")
	authHeader       = os.Getenv("AUTH_HEADER")
	authHeaderPrefix = os.Getenv("AUTH_HEADER_PREFIX")
)

var tokens = map[string]string{
	http.MethodGet:    os.Getenv("TOKEN_GET"),
	http.MethodPut:    os.Getenv("TOKEN_PUT"),
	http.MethodDelete: os.Getenv("TOKEN_DELETE"),
}

func init() {
	if rootDir == "" {
		rootDir = "/uploads"
	}

	err := os.MkdirAll(rootDir, os.ModePerm)
	if err != nil {
		log.Fatal(err)
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

	if authHeader == "" {
		authHeader = "Authorization"
	}

	if authHeaderPrefix == "" {
		authHeaderPrefix = "Bearer "
	}
}

func main() {
	log.Info("Staring simple upload server.")
	log.Panic(http.ListenAndServe(listen, http.HandlerFunc(handleRoot)))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.URL.Path == urlBasePath && r.Method != http.MethodGet {
		log.Debugf("Unsupported method %s for /", r.Method)
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)

		return
	}

	if tokens[r.Method] != "" {
		token := r.URL.Query().Get("token")

		if _, ok := r.Header[authHeader]; ok && token == "" {
			if !strings.HasPrefix(r.Header.Get(authHeader), authHeaderPrefix) {
				log.Debugf("Wrong header prefix for %s: %s", r.Method, r.URL.Path)
				http.Error(w, "Wrong header prefix", http.StatusUnauthorized)

				return
			}

			token = strings.TrimPrefix(r.Header.Get(authHeader), authHeaderPrefix)
		}

		if token == "" {
			log.Debugf("Missing token for %s: %s", r.Method, r.URL.Path)
			http.Error(w, "Missing token", http.StatusUnauthorized)

			return
		}

		if tokens[r.Method] != token {
			log.Debugf("Wrong token for %s: %s", r.Method, r.URL.Path)
			http.Error(w, "Wrong token", http.StatusUnauthorized)

			return
		}
	}

	log.Debugf("%s: %s", r.Method, r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		http.StripPrefix(urlBasePath, http.FileServer(http.Dir(rootDir))).ServeHTTP(w, r)
	case http.MethodPut:
		handlePut(w, r)
	case http.MethodDelete:
		handleDelete(w, r)
	default:
		log.Debug("Unsupported method %s for %s", r.Method, r.URL.Path)
		w.Header().Set("Allow", strings.Join([]string{http.MethodGet, http.MethodPut, http.MethodDelete}, ", "))
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
