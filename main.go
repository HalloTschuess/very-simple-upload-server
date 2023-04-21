package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"hash"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
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
	forceDigest, _   = strconv.ParseBool(os.Getenv("FORCE_DIGEST"))
)

var tokens = map[string]string{
	http.MethodGet:    os.Getenv("TOKEN_GET"),
	http.MethodPut:    os.Getenv("TOKEN_PUT"),
	http.MethodDelete: os.Getenv("TOKEN_DELETE"),
}

var allowedMethods = []string{http.MethodOptions, http.MethodGet, http.MethodPut, http.MethodDelete}

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
	case http.MethodOptions:
		w.Header().Set("Allow", strings.Join(allowedMethods, ", "))
	case http.MethodGet:
		http.StripPrefix(urlBasePath, http.FileServer(http.Dir(rootDir))).ServeHTTP(w, r)
	case http.MethodPut:
		handlePut(w, r)
	case http.MethodDelete:
		handleDelete(w, r)
	default:
		log.Debugf("Unsupported method %s for %s", r.Method, r.URL.Path)
		w.Header().Set("Allow", strings.Join(allowedMethods, ", "))
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
	}
}

func handlePut(w http.ResponseWriter, r *http.Request) {
	w = &ignoreMultiWriteHeaderWriter{ResponseWriter: w}
	defer w.WriteHeader(http.StatusNoContent)

	digest, ok, err := parseDigest(r.Header.Get("Digest"))
	if err != nil {
		log.Errorf("Error while parsing digest: %s", err)
		http.Error(w, "Digest could not be parsed \n\n"+err.Error(), http.StatusBadRequest)
		return
	}
	if forceDigest && !ok {
		log.Errorf("Missing digest")
		http.Error(w, "Missing digest. Supported algorithms: sha-256, md5", http.StatusBadRequest)
		return
	}

	var fileReader io.Reader

	formFile, _, err := r.FormFile("file")
	if err != nil {
		fileReader = r.Body
	} else {
		defer formFile.Close()
		fileReader = formFile
	}

	subjectPath := urlToPath(r.URL.Path)

	err = os.MkdirAll(path.Dir(subjectPath), 0700)
	if err != nil {
		log.Warnf("Error while creating directory for file '%s': %s'", subjectPath, err)
		http.Error(w, "Path could not be created. Make sure the path is correct", http.StatusInternalServerError)
		return
	}
	var rollback multiFunc
	rollback.add(func() {
		if err := cleanEmptyDir(rootDir, filepath.Dir(subjectPath)); err != nil {
			log.Errorf("Error cleaning empty directories: %s", err)
		}
	})

	file, err := newTransactionFile(subjectPath)
	if err != nil {
		log.Errorf("Error while opening file '%s': %s", subjectPath, err)
		http.Error(w, "File could not be created/opened", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Errorf("Error while closing file '%s': %s", subjectPath, err)
			http.Error(w, "File could not be closed", http.StatusInternalServerError)
		}
	}()
	rollback.add(func() {
		if err := file.rollback(); err != nil {
			log.Errorf("Error while rolling back file '%s': %s", subjectPath, err)
		}
	})

	out := io.MultiWriter(file, digest)

	_, err = io.Copy(out, fileReader)
	if err != nil {
		log.Errorf("Error while writing file '%s': %s", subjectPath, err)
		http.Error(w, "File could not be saved", http.StatusInternalServerError)
		rollback.run()
		return
	}

	if !digest.isValid() {
		log.Warnf("Invalid digest for file '%s'", subjectPath)
		http.Error(w, "Invalid digest. Supported algorithms: sha-256, md5", http.StatusBadRequest)
		rollback.run()
		return
	}

	log.Debugf("File '%s' written", subjectPath)
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

	if err := cleanEmptyDir(rootDir, filepath.Dir(subjectPath)); err != nil {
		log.Errorf("Error cleaning empty directories: %s", err)
		http.Error(w, "Could clean empty directories", http.StatusInternalServerError)
		return
	}

	log.Debugf("File '%s' deleted", subjectPath)

	w.WriteHeader(http.StatusNoContent)
}

func urlToPath(urlPath string) string {
	return path.Join(rootDir, strings.TrimPrefix(path.Clean(urlPath), urlBasePath))
}

func cleanEmptyDir(root, path string) error {
	for !isPathEqual(root, path) {
		empty, err := isDirEmpty(path)
		if err != nil {
			return fmt.Errorf("empty directory check '%s': %w", path, err)
		}

		if !empty {
			return nil
		}

		log.Debug("Delete parent dir ")
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("delete directory '%s': %s", path, err)
		}
		path = filepath.Dir(path)
	}
	return nil
}

func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("directory open: %w", err)
	}
	defer f.Close()
	_, err = f.ReadDir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func isPathEqual(path1, path2 string) bool {
	p1, err := filepath.Abs(path1)
	if err != nil {
		return false
	}
	p2, err := filepath.Abs(path2)
	if err != nil {
		return false
	}
	return p1 == p2
}

type transactionFile struct {
	*os.File
	target string
	done   bool
}

func newTransactionFile(p string) (*transactionFile, error) {
	f, err := os.CreateTemp(filepath.Dir(p), "")
	if err != nil {
		return nil, err
	}
	return &transactionFile{
		File:   f,
		target: p,
	}, nil
}

func (f *transactionFile) Close() error {
	return f.commit()
}

func (f *transactionFile) commit() error {
	if f.done {
		return nil
	}
	f.done = true
	if err := f.File.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	err := os.Rename(f.File.Name(), f.target)
	if err == nil {
		return nil
	}
	if !os.IsExist(err) {
		return fmt.Errorf("rename temp file: %w", err)
	}

	if err := os.Remove(f.target); err != nil {
		return fmt.Errorf("delete target file: %w", err)
	}
	if err := os.Rename(f.File.Name(), f.target); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func (f *transactionFile) rollback() error {
	if f.done {
		return nil
	}
	f.done = true
	if err := f.File.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Remove(f.File.Name()); err != nil {
		return fmt.Errorf("remove temp file: %w", err)
	}
	return nil
}

type ignoreMultiWriteHeaderWriter struct {
	http.ResponseWriter
	headerWritten bool
}

func (w *ignoreMultiWriteHeaderWriter) WriteHeader(code int) {
	if w.headerWritten {
		return
	}
	w.ResponseWriter.WriteHeader(code)
	w.headerWritten = true
}

type multiFunc struct {
	fns []func()
}

func (r *multiFunc) add(fn func()) {
	r.fns = append(r.fns, fn)
}

func (r *multiFunc) run() {
	for _, fn := range r.fns {
		fn()
	}
}

type digestValidator interface {
	io.Writer
	isValid() bool
}

var digestValidators = map[string]func(string) (digestValidator, error){
	"sha-256": newSHA256Validator,
	"md5":     newMD5Validator,
}

func parseDigest(s string) (digestValidator, bool, error) {
	parts := strings.Split(s, ",")
	validator := &multiDigestValidator{
		Writer:     io.MultiWriter(),
		validators: make([]digestValidator, 0, len(parts)),
	}

	var errs []error
	for _, part := range parts {
		algo, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}

		if fn, ok := digestValidators[algo]; ok {
			v, err := fn(val)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			validator.Writer = io.MultiWriter(validator.Writer, v)
			validator.validators = append(validator.validators, v)
		}
	}

	return validator, len(validator.validators) > 0, errors.Join(errs...)
}

type multiDigestValidator struct {
	io.Writer
	validators []digestValidator
}

func (v *multiDigestValidator) isValid() bool {
	for _, validator := range v.validators {
		if !validator.isValid() {
			return false
		}
	}
	return true
}

type hashValidator struct {
	digest []byte
	hasher hash.Hash
}

func (v *hashValidator) Write(p []byte) (int, error) {
	return v.hasher.Write(p)
}

func (v *hashValidator) isValid() bool {
	sum := v.hasher.Sum(nil)
	if bytes.Equal(sum, v.digest) {
		return true
	}
	return false
}

func newSHA256Validator(digest string) (digestValidator, error) {
	d, err := base64.StdEncoding.DecodeString(digest)
	if err != nil {
		return nil, err
	}
	return &hashValidator{
		digest: d,
		hasher: sha256.New(),
	}, nil
}

func newMD5Validator(digest string) (digestValidator, error) {
	d, err := base64.StdEncoding.DecodeString(digest)
	if err != nil {
		return nil, err
	}
	return &hashValidator{
		digest: d,
		hasher: md5.New(),
	}, nil
}
