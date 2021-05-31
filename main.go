package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-playground/pure/v5"
	"github.com/go-playground/pure/v5/middleware"
)

var scheme *regexp.Regexp = regexp.MustCompile(`^http(?:s)?:\/\/`)

func main() {
	p := pure.New()

	p.Get("/rss/*", middleware.Gzip(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL.Path)
		feedUrl := r.URL.Path[5:]
		prepend := ""
		if !scheme.MatchString(feedUrl) {
			prepend = "https://"
		}
		resp, err := http.Get(prepend + feedUrl)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		if resp.StatusCode != http.StatusOK {
			http.Error(w, fmt.Sprintf("upstream server returned status %d instead of %d", resp.StatusCode, http.StatusOK), http.StatusBadGateway)
			return
		}

		if !strings.Contains(resp.Header.Get("Content-Type"), "xml") {
			http.Error(w, "upstream server did not purport to return an XML document", http.StatusBadGateway)
			return
		}

		decoder := xml.NewDecoder(resp.Body)
		var buf bytes.Buffer
		encoder := xml.NewEncoder(&buf)

		scheme := "http://"
		if r.TLS != nil {
			scheme = "https://"
		}

		var token xml.Token
		for token, err = decoder.RawToken(); err == nil; token, err = decoder.Token() {
			if start, ok := token.(xml.StartElement); ok {
				if start.Name.Space == "" && start.Name.Local == "enclosure" {
					start = start.Copy()
					newAttr := make([]xml.Attr, 0, len(start.Attr))
					for _, a := range start.Attr {
						if a.Name.Space == "" {
							switch a.Name.Local {
							case "url":
								newAttr = append(newAttr, xml.Attr{
									Name:  a.Name,
									Value: scheme + r.Host + "/media/" + a.Value,
								})
								continue
							case "type":
								newAttr = append(newAttr, xml.Attr{
									Name:  a.Name,
									Value: "audio/ogg;codecs=opus",
								})
								continue
							case "length":
								continue
							}
						}
						newAttr = append(newAttr, a)
					}
					start.Attr = newAttr
					token = start
				}
			}
			encoder.EncodeToken(token)
		}

		if err != io.EOF {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		encoder.Flush()
		w.Header().Add("Content-Type", "application/rss+xml")
		w.Header().Add("Content-Length", strconv.Itoa(buf.Len()))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, &buf)
	}))

	p.Get("/media/*", func(w http.ResponseWriter, r *http.Request) {
		mediaUrl := r.URL.Path[7:]
		w.Header().Add("Content-Type", "audio/ogg;codecs=opus")

		var stderr bytes.Buffer
		cmd := exec.Command("ffmpeg", "-i", mediaUrl, "-f", "ogg", "-vn", "-acodec", "libopus", "-b:a", "64000", "-")
		cmd.Stdout = w
		cmd.Stderr = &stderr
		err := cmd.Run()

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if cmd.ProcessState.ExitCode() != 0 {
			http.Error(w, stderr.String(), http.StatusInternalServerError)
			return
		}
	})

	log.Println("Ready to serve")
	http.ListenAndServe("127.0.0.1:5000", p.Serve())
}
