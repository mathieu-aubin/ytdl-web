package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const cleanupInterval = 30 // seconds

func main() {

	ytCmd := flag.String("cmd", "/usr/bin/youtube-dl", "path to youtube-dl")
	webRoot := flag.String("webRoot", "html", "web root directory")
	outPath := flag.String("outPath", "dl", "where to store downloaded files (relative to web root)")
	timeout := flag.Int("timeout", DefaultProcessTimeout, "process timeout (seconds)")
	expiry := flag.Int("expiry", DefaultExpiry, "expire downloaded content (seconds)")
	port := flag.Int("port", 8080, "listen on this port")
	flag.Parse()

	log.Printf("Starting ytdl-web...\n")
	log.Printf("Set web root: %s\n", *webRoot)
	log.Printf("Set process timeout: %d sec\n", *timeout)
	log.Printf("Set output path: %s\n", *webRoot+"/"+*outPath)
	log.Printf("Set content expiry: %d sec\n", *expiry)

	ws := &wsHandler{
		WebRoot: *webRoot,
		Timeout: time.Duration(*timeout) * time.Second,
		YTCmd:   *ytCmd,
		OutPath: *outPath,
	}
	http.Handle("/websocket", ws)

	changeHeaderThenServe := func(h http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".webm") || strings.HasSuffix(r.URL.Path, ".m4a") || strings.HasSuffix(r.URL.Path, ".mp3") {
				w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path.Base(r.URL.Path)))
			}
			h.ServeHTTP(w, r)
		}
	}

	http.Handle("/", changeHeaderThenServe(http.FileServer(http.Dir(*webRoot))))

	log.Printf("Starting cleanup routine...\n")
	expiryD := time.Second * time.Duration(*expiry)
	go fileCleanup(*webRoot+"/"+*outPath, expiryD)

	log.Printf("Listening on :%d...\n", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatal(err)
	}

}

func fileCleanup(outPath string, expiry time.Duration) {
	visit := func(path string, f os.FileInfo, err error) error {

		if err != nil {
			return err
		}

		if f.IsDir() || strings.HasPrefix(f.Name(), ".") {
			return nil
		}

		// if last modification time is prior to expiry time,
		// then delete the file
		if time.Now().Sub(f.ModTime()) > expiry {
			if err := os.Remove(path); err != nil {
				return err
			}
			log.Printf("old file removed: %s\n", path)
		}
		return nil
	}

	tickChan := time.NewTicker(time.Second * cleanupInterval).C

	for _ = range tickChan {
		err := filepath.Walk(outPath, visit)
		if err != nil {
			log.Printf("file cleanup error: %s\n", err)
		}
	}
}
