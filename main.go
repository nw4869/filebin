package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/GeertJohan/go.rice"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/nw4869/filebin/app/api"
	"github.com/nw4869/filebin/app/backend/fs"
	"github.com/nw4869/filebin/app/config"
	"github.com/nw4869/filebin/app/events"
	"github.com/nw4869/filebin/app/metrics"
	"github.com/nw4869/filebin/app/model"
)

var cfg = config.Global
var githash = "No githash provided"
var buildstamp = "No buildstamp provided"

var staticBox *rice.Box
var templateBox *rice.Box
var backend fs.Backend
var m metrics.Metrics
var e events.Events

// Initiate buffered channel for batch processing
var WorkQueue = make(chan model.Job, 1000)

func isDir(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	if fi.IsDir() {
		return true
	} else {
		return false
	}
}

func generateReqId(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())

	flag.StringVar(&cfg.Baseurl, "baseurl",
		cfg.Baseurl, "Baseurl used when generating links.")

	flag.StringVar(&cfg.Filedir, "filedir",
		cfg.Filedir, "Directory to store uploaded files.")

	flag.StringVar(&cfg.Tempdir, "tempdir",
		cfg.Tempdir, "Directory to temporarily store files during upload.")

	//flag.StringVar(&cfg.Logdir, "logdir",
	//	cfg.Logdir, "Directory to write log files to.")

	//flag.StringVar(&cfg.Thumbdir, "thumbdir",
	//	cfg.Thumbdir, "Path to thumbnail directory")

	//flag.StringVar(&cfg.Database, "database",
	//	cfg.Database, "Path to database file")

	flag.StringVar(&cfg.Host, "host",
		cfg.Host, "Listen host.")

	flag.IntVar(&cfg.Port, "port",
		cfg.Port, "Listen port.")

	flag.IntVar(&cfg.ReadTimeout, "readtimeout",
		cfg.ReadTimeout, "Request read timeout in seconds.")

	flag.IntVar(&cfg.WriteTimeout, "writetimeout",
		cfg.WriteTimeout, "Response write timeout in seconds.")

	flag.IntVar(&cfg.MaxHeaderBytes, "maxheaderbytes",
		cfg.MaxHeaderBytes, "Max header size in bytes.")

	flag.StringVar(&cfg.ClientAddrHeader, "client-address-header",
		cfg.ClientAddrHeader, "Read the client address from the specified request header instad of using the connection remote address.")

	flag.BoolVar(&cfg.CacheInvalidation, "cache-invalidation",
		cfg.CacheInvalidation,
		"HTTP PURGE requests will be sent on every change if enabled.")

	flag.IntVar(&cfg.Workers, "workers",
		cfg.Workers, "Number of workers for background processing.")

	//flag.IntVar(&cfg.Pagination, "pagination",
	//	cfg.Pagination,
	//	"Files to show per page for pagination.")

	//flag.StringVar(&cfg.GeoIP2, "geoip2",
	//	cfg.GeoIP2, "Path to the GeoIP2 database file.")

	flag.Int64Var(&cfg.Expiration, "expiration",
		cfg.Expiration, "Bin expiration time in seconds after the last modification.")

	//flag.BoolVar(&cfg.Verbose, "verbose",
	//	cfg.Verbose, "Verbose output.")

	flag.StringVar(&cfg.TriggerNewBin, "trigger-new-bin",
		cfg.TriggerNewBin,
		"Command to execute when a bin is created.")

	flag.StringVar(&cfg.TriggerUploadFile,
		"trigger-upload-file",
		cfg.TriggerUploadFile,
		"Command to execute when a file is uploaded.")

	flag.StringVar(&cfg.TriggerDownloadBin,
		"trigger-download-bin",
		cfg.TriggerDownloadBin,
		"Command to execute when a bin archive is downloaded.")

	flag.StringVar(&cfg.TriggerDownloadFile,
		"trigger-download-file",
		cfg.TriggerDownloadFile,
		"Command to execute when a file is downloaded.")

	flag.StringVar(&cfg.TriggerDeleteBin,
		"trigger-delete-bin",
		cfg.TriggerDeleteBin,
		"Command to execute when a bin is deleted.")

	flag.StringVar(&cfg.TriggerDeleteFile,
		"trigger-delete-file",
		cfg.TriggerDeleteFile,
		"Command to execute when a file is deleted.")

	flag.StringVar(&cfg.AdminUsername,
		"admin-username",
		cfg.AdminUsername,
		"Administrator username.")

	flag.StringVar(&cfg.AdminPassword,
		"admin-password",
		cfg.AdminPassword,
		"Administrator password.")

	flag.StringVar(&cfg.AccessLog,
		"access-log",
		cfg.AccessLog,
		"Path to combined format access log file.")

	//	flag.StringVar(&cfg.TriggerExpiredBin, "trigger-expired-bin",
	//		cfg.TriggerExpiredBin,
	//		"Trigger to execute when a bin expires.")

	flag.BoolVar(&cfg.Version, "version",
		cfg.Version, "Show the version information.")

	flag.Parse()

	if cfg.Version {
		fmt.Println("Git Commit Hash: " + githash)
		fmt.Println("UTC Build Time: " + buildstamp)
		os.Exit(0)
	}

	//if (!IsDir(cfg.Logdir)) {
	//    fmt.Println("The specified log directory is not a directory: ",
	//        cfg.Logdir)
	//    os.Exit(2)
	//}

	if cfg.Port < 1 || cfg.Port > 65535 {
		log.Fatalln("Invalid port number, aborting.")
	}

	if cfg.ReadTimeout < 1 || cfg.ReadTimeout > 86400 {
		log.Fatalln("Invalid read timeout, aborting.")
	}

	if cfg.WriteTimeout < 1 || cfg.WriteTimeout > 86400 {
		log.Fatalln("Invalid write timeout, aborting.")
	}

	if cfg.MaxHeaderBytes < 1 ||
		cfg.MaxHeaderBytes > 2<<40 {
		log.Fatalln("Invalid max header bytes, aborting.")
	}

	if !isDir(cfg.Tempdir) {
		log.Fatalln("The directory " + cfg.Tempdir + " does not exist.")
	}

	//if _, err := os.Stat(cfg.GeoIP2); err == nil {
	//    gi, err = geoip2.Open(cfg.GeoIP2)
	//    if err != nil {
	//        Info.Print("Could not open GeoIP2 database ", cfg.GeoIP2,
	//            ": ", err)
	//    }
	//    defer gi.Close()
	//} else {
	//    Info.Print("GeoIP2 database does not exist: ", cfg.GeoIP2)
	//}
}

func main() {
	var err error
	log := log.New(os.Stdout, "- ", log.LstdFlags)

	// Initialize boxes
	staticBox = rice.MustFindBox("static")
	templateBox = rice.MustFindBox("templates")

	log.Println("Listen host: " + cfg.Host)
	log.Println("Listen port: " + strconv.Itoa(cfg.Port))
	log.Println("Read timeout: " +
		strconv.Itoa(cfg.ReadTimeout) + " seconds")
	log.Println("Write timeout: " +
		strconv.Itoa(cfg.WriteTimeout) + " seconds")
	log.Println("Max header size: " +
		strconv.Itoa(cfg.MaxHeaderBytes) + " bytes")
	log.Println("Access log file: " + cfg.AccessLog)
	log.Println("Cache invalidation enabled: " +
		strconv.FormatBool(cfg.CacheInvalidation))
	log.Println("Workers: " +
		strconv.Itoa(cfg.Workers))
	log.Println("Expiration time: " +
		strconv.FormatInt(cfg.Expiration, 10) + " seconds")
	log.Println("Files directory: " + cfg.Filedir)
	log.Println("Temp directory: " + cfg.Tempdir)
	//log.Println("Log directory: " + cfg.Logdir)
	log.Println("Baseurl: " + cfg.Baseurl)

	var trigger = cfg.TriggerNewBin
	if trigger == "" {
		trigger = "Not set"
	}
	log.Println("Trigger - New bin: " + trigger)

	trigger = cfg.TriggerUploadFile
	if trigger == "" {
		trigger = "Not set"
	}
	log.Println("Trigger - Upload file: " + trigger)

	trigger = cfg.TriggerDownloadBin
	if trigger == "" {
		trigger = "Not set"
	}
	log.Println("Trigger - Download bin: " + trigger)

	trigger = cfg.TriggerDownloadFile
	if trigger == "" {
		trigger = "Not set"
	}
	log.Println("Trigger - Download file: " + trigger)

	trigger = cfg.TriggerDeleteBin
	if trigger == "" {
		trigger = "Not set"
	}
	log.Println("Trigger - Delete bin: " + trigger)

	trigger = cfg.TriggerDeleteFile
	if trigger == "" {
		trigger = "Not set"
	}
	log.Println("Trigger - Delete file: " + trigger)

	//fmt.Println("Trigger Expired bin: " + cfg.TriggerExpiredBin)

	accessLogWriter, err := os.OpenFile(cfg.AccessLog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Unable to open access log file: " + err.Error())
		os.Exit(2)
	}

	m = metrics.Init()

	startTime := time.Now().UTC()
	backend, err = fs.InitBackend(cfg.Baseurl, cfg.Filedir, cfg.Tempdir, cfg.Expiration, log)
	if err != nil {
		log.Fatalln(err.Error())
	}
	finishTime := time.Now().UTC()
	elapsedTime := finishTime.Sub(startTime)
	backend.Log.Println("Backend initialized in: " + elapsedTime.String())

	log.Println("Backend: " + backend.Info())
	log.Println("Filebin server starting...")

	// Start dispatcher that will handle all background processing
	model.StartDispatcher(cfg.Workers, WorkQueue, &backend)

	// Sending all files through the batch process to ensure thumbnails
	// are generated.
	for _, bin := range backend.GetBins() {
		files := backend.GetFiles(bin)
		for _, filename := range files {
			j := model.Job{}
			j.Filename = filename
			j.Bin = bin
			j.Log = log
			j.Cfg = &cfg
			WorkQueue <- j
		}
	}

	router := mux.NewRouter()

	router.Handle("/static/{path:.*}", http.StripPrefix("/static/", http.FileServer(staticBox.HTTPBox()))).Methods("GET", "HEAD")

	http.Handle("/", httpInterceptor(router))

	// Accept trailing slashes.
	// Disabling this feature for now since it might not be needed. Try to
	// find some other way of accepting trailing slashes where appropriate
	// instead of globally.
	//router.StrictSlash(true)

	// Skip reqHandler to avoid logging of requests to this endpoint
	router.HandleFunc("/filebin-status", api.FilebinStatus).Methods("GET", "HEAD")

	router.HandleFunc("/admin", basicAuth(reqHandler(api.AdminDashboard))).Methods("GET", "HEAD")
	router.HandleFunc("/admin/events", basicAuth(reqHandler(api.AdminEvents))).Methods("GET", "HEAD")
	router.HandleFunc("/admin/bins", basicAuth(reqHandler(api.AdminBins))).Methods("GET", "HEAD")
	router.HandleFunc("/admin/counters", basicAuth(reqHandler(api.AdminCounters))).Methods("GET", "HEAD")
	router.HandleFunc("/readme", reqHandler(api.Readme)).Methods("GET", "HEAD")
	router.HandleFunc("/", reqHandler(api.NewBin)).Methods("GET", "HEAD")
	router.HandleFunc("/", reqHandler(api.Upload)).Methods("POST")
	router.HandleFunc("/archive/{bin:[A-Za-z0-9_-]+}/{format:[a-z]+}", reqHandler(api.FetchArchive)).Methods("GET", "HEAD")
	router.HandleFunc("/album/{bin:[A-Za-z0-9_-]+}", reqHandler(api.FetchAlbum)).Methods("GET", "HEAD")
	router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", reqHandler(api.FetchBin)).Methods("GET", "HEAD")
	router.HandleFunc("/{bin:[A-Za-z0-9_-]+}", reqHandler(api.DeleteBin)).Methods("DELETE")
	router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", reqHandler(api.FetchFile)).Methods("GET", "HEAD")
	router.HandleFunc("/{bin:[A-Za-z0-9_-]+}/{filename:.+}", reqHandler(api.DeleteFile)).Methods("DELETE")
	router.HandleFunc("/{path:.*}", reqHandler(api.PurgeHandler)).Methods("PURGE")

	logRouter := handlers.CombinedLoggingHandler(accessLogWriter, router)

	server := &http.Server{
		Addr:           cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Handler:        logRouter,
		ReadTimeout:    time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.WriteTimeout) * time.Second,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Panicln(err.Error())
	}
}

func reqHandler(fn func(http.ResponseWriter, *http.Request, config.Configuration, model.Context)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now().UTC()
		reqId := "r-" + generateReqId(5)

		// Populate the context for this request here
		var ctx = model.Context{}
		ctx.TemplateBox = templateBox
		ctx.StaticBox = staticBox
		ctx.Baseurl = cfg.Baseurl
		ctx.WorkQueue = WorkQueue
		ctx.Backend = &backend
		ctx.Metrics = &m
		ctx.Events = &e

		if cfg.ClientAddrHeader == "" {
			// Extract the IP address only
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err == nil {
				ctx.RemoteAddr = ip
			} else {
				ctx.RemoteAddr = r.RemoteAddr
			}
		} else {
			ctx.RemoteAddr = r.Header.Get(cfg.ClientAddrHeader)
		}

		// Initialize logger for this request
		ctx.Log = log.New(os.Stdout, reqId+" ", log.LstdFlags)

		ctx.Log.Println(r.Method + " " + r.RequestURI)
		if r.Host != "" {
			ctx.Log.Println("Host: " + r.Host)
		}
		ctx.Log.Println("Remote address: " + ctx.RemoteAddr)

		// Print X-Forwarded-For since we might be behind some TLS
		// terminator and web cache
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" && xff != ctx.RemoteAddr {
			ctx.Log.Println("X-Forwarded-For: " + xff)
		}

		referer := r.Header.Get("Referer")
		if referer != "" {
			ctx.Log.Println("Referer: " + referer)
		}

		ua := r.Header.Get("User-Agent")
		if ua != "" {
			ctx.Log.Println("User-Agent: " + ua)
		}

		fn(w, r, cfg, ctx)

		finishTime := time.Now().UTC()
		elapsedTime := finishTime.Sub(startTime)
		ctx.Log.Println("Response time: " + elapsedTime.String())
	}
}

func basicAuth(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Let the client know authentication is required
		w.Header().Set("WWW-Authenticate", "Basic realm='Filebin'")

		// Abort here if the admin username or password is not set
		if cfg.AdminUsername == "" || cfg.AdminPassword == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Read the authorization request header
		auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(auth) != 2 || auth[0] != "Basic" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		payload, _ := base64.StdEncoding.DecodeString(auth[1])
		pair := strings.SplitN(string(payload), ":", 2)

		if len(pair) != 2 {
			http.Error(w, "authorization failed", http.StatusUnauthorized)
			return
		}

		// Verify that the username and password match
		username := pair[0]
		password := pair[1]
		if username != cfg.AdminUsername || password != cfg.AdminPassword {
			time.Sleep(3 * time.Second)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		fn(w, r)
	}
}

func httpInterceptor(router http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router.ServeHTTP(w, r)
	})
}
