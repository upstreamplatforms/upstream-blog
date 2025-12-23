package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var siteConfig Config
var index Index
var viewIndex = ""
var viewNew = ""
var viewView = ""
var staticCache map[string]string = make(map[string]string)
var dataDir = "./data"

func main() {

	Init()

	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-signalChannel
		switch sig {
		case os.Interrupt:
			fmt.Println("Interrupt received, persisting index and closing.")
			//content.Finalize()
			os.Exit(1)
		case syscall.SIGKILL:
			fmt.Println("SIGINT received, persisting index and closing.")
			//content.Finalize()
			os.Exit(1)
		case syscall.SIGTERM:
			fmt.Println("SIGTERM received, persisting index and closing.")
			//content.Finalize()
			os.Exit(1)
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, viewIndex)
	})

	mux.HandleFunc("GET /config", func(w http.ResponseWriter, r *http.Request) {
		config := map[string]string{
			"authApiKey": os.Getenv("AUTH_API_KEY"),
			"authDomain": os.Getenv("AUTH_DOMAIN"),
		}
		b, _ := json.MarshalIndent(config, "", " ")
		fmt.Fprint(w, string(b))
	})

	mux.HandleFunc("GET /new", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, viewNew)
	})

	mux.HandleFunc("GET /rss", func(w http.ResponseWriter, r *http.Request) {
		var sb strings.Builder
		sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\" ?>\n")
		sb.WriteString("  <rss version=\"2.0\">\n")
		sb.WriteString("  <channel>\n")
		sb.WriteString("    <title>Upstream Utopia</title>\n")
		sb.WriteString("    <link>https://upstreamutopia.com</link>\n")
		sb.WriteString("    <description>To Deepen Understanding</description>\n")

		postIndex := len(index.Posts) - 1
		if postIndex >= 0 {
			for postIndex >= 0 {
				pubDate := time.Unix(index.Posts[postIndex].Date, 0)

				sb.WriteString("    <item>\n")
				sb.WriteString("      <title>")
				sb.WriteString(index.Posts[postIndex].Title)
				sb.WriteString("</title>\n")
				sb.WriteString("      <link>https://upstreamutopia.com/")
				sb.WriteString(index.Posts[postIndex].Id)
				sb.WriteString("</link>\n")
				sb.WriteString("      <description>")
				sb.WriteString(index.Posts[postIndex].Excerpt)
				sb.WriteString("</description>\n")
				sb.WriteString("      <pubDate>")
				sb.WriteString(pubDate.String())
				sb.WriteString("</pubDate>\n")
				sb.WriteString("    </item>\n")

				postIndex = postIndex - 1
			}
		}

		sb.WriteString("  </channel>\n")
		sb.WriteString("</rss>\n")

		//fmt.Println(sb.String())
		fmt.Fprint(w, sb.String())
	})

	mux.HandleFunc("GET /static/{file}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("file")
		val, ok := staticCache[name]
		if !ok {
			b, err := os.ReadFile("./views/static/" + name)
			if err == nil {
				val = string(b)
				staticCache[name] = val
			}
		}
		if val != "" {
			contentType := "text/css"
			if strings.HasSuffix(name, ".js") {
				contentType = "application/js"
			}
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, val)

		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	mux.HandleFunc("GET /{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, viewView)
	})

	mux.HandleFunc("GET /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		b, err := os.ReadFile(dataDir + "/posts/" + id + ".json")
		if err == nil {
			val := string(b)
			fmt.Fprint(w, val)
		}
	})

	mux.HandleFunc("POST /posts", func(w http.ResponseWriter, r *http.Request) {
		// validate id token

		var newContent []byte
		// contentType := r.Header.Get("Content-Type")

		// if strings.HasPrefix(contentType, "multipart/form-data") {
		// 	var files []multipart.FileHeader
		// 	form, _ := r.Form()
		// 	if form != nil && form.File != nil && form.File["files"] != nil {
		// 		for _, file := range form.File["files"] {
		// 			files = append(files, *file)
		// 		}
		// 	}
		// 	var inputMap map[string]string = make(map[string]string)
		// 	for key, value := range r.PostForm {
		// 		fmt.Printf("%v = %v \n", key, value)
		// 		inputMap[key] = value[0]
		// 	}
		// 	newContent, _ = json.MarshalIndent(inputMap, "", "  ")
		// } else {
		var err error
		newContent, err = io.ReadAll(r.Body)
		if err != nil {
			log.Println(err.Error())
		}
		//}

		var newPost Post
		json.Unmarshal(newContent, &newPost)
		lastIdPiece := strings.ReplaceAll(strings.ToLower(newPost.Title), " ", "-")
		if len(lastIdPiece) > 27 {
			lastIdPiece = lastIdPiece[0:27]
		}
		newPost.Id = time.Now().Format("20060102-150405-") + lastIdPiece
		newPost.Date = time.Now().Unix()
		if newPost.Category == "" {
			newPost.Category = "General"
		}
		newPost.ReadTime = "4 min"
		// if len(newPost.Content) > 40 {
		// 	newPost.Excerpt = newPost.Content[:40]
		// } else {
		// 	newPost.Excerpt = newPost.Content
		// }
		newPost.Image = "https://picsum.photos/200/300" // "linear-gradient(135deg, #fce7f3 0%, #fbcfe8 100%)"
		postBytes, _ := json.MarshalIndent(newPost, "", "  ")
		os.WriteFile(dataDir+"/posts/"+newPost.Id+".json", postBytes, 0644)

		// update index
		postMeta := PostMeta{newPost.Id, newPost.Title, newPost.Excerpt, newPost.Category, newPost.Date, newPost.ReadTime, newPost.Image, newPost.Tags}
		index.Posts = append(index.Posts, postMeta)

		if len(index.Posts) >= siteConfig.IndexSize*2 {
			previousIndex := Index{}
			previousIndex.Posts = make([]PostMeta, 0, siteConfig.IndexSize)
			previousIndex.Posts = append(previousIndex.Posts, index.Posts[0:siteConfig.IndexSize]...)
			b, _ := json.MarshalIndent(previousIndex, "", " ")
			os.WriteFile(dataDir+"/index_"+strconv.Itoa(siteConfig.IndexCount)+".json", b, 0644)
			// newPostsIndex := make([]PostMeta, 0, siteConfig.IndexSize*2)
			// newPostsIndex = append(newPostsIndex, index.Posts[siteConfig.IndexSize+1:]...)
			index.Posts = slices.Delete(index.Posts, 0, siteConfig.IndexSize)
			siteConfig.IndexCount++
			b, _ = json.MarshalIndent(siteConfig, "", " ")
			os.WriteFile(dataDir+"/config.json", b, 0644)
		}

		go persistIndex()

		fmt.Fprint(w, "OK")
	})

	mux.HandleFunc("GET /posts", func(w http.ResponseWriter, r *http.Request) {
		// startIndex := 0
		// pageSize := 10
		// params, _ := url.ParseQuery(r.URL.RawQuery)

		// if len(params["startIndex"]) > 0 {
		// 	startIndex, _ = strconv.Atoi(params["startIndex"][0])
		// }
		// if len(params["pageSize"]) > 0 {
		// 	pageSize, _ = strconv.Atoi(params["pageSize"][0])
		// }

		// fmt.Println(startIndex)
		// fmt.Println(pageSize)

		b, _ := json.MarshalIndent(index.Posts, "", " ")
		fmt.Fprint(w, string(b))
	})

	log.Print("Listening...")

	http.ListenAndServe(":8080", mux)
}

func Init() {
	tempDataDir := os.Getenv("DATA_DIR")
	if tempDataDir != "" {
		dataDir = tempDataDir
	}
	os.MkdirAll(dataDir+"/posts", os.ModePerm)

	// load config
	b, err := os.ReadFile(dataDir + "/config.json")
	if err != nil {
		b, err = os.ReadFile("./config.json")
		json.Unmarshal(b, &siteConfig)
		os.WriteFile(dataDir+"/config.json", b, 0644)
	} else {
		json.Unmarshal(b, &siteConfig)
	}

	// load most recent index
	b, err = os.ReadFile(dataDir + "/index_" + strconv.Itoa(siteConfig.IndexCount) + ".json")
	if err != nil {
		index = Index{}
		index.Posts = make([]PostMeta, 0, siteConfig.IndexSize)
		b, _ = json.MarshalIndent(index, "", " ")
		os.WriteFile(dataDir+"/index_"+strconv.Itoa(siteConfig.IndexCount)+".json", b, 0644)
	} else {
		json.Unmarshal(b, &index)
	}

	// load views
	b, err = os.ReadFile("./views/index.html")
	if err == nil {
		viewIndex = string(b)
	}

	b, err = os.ReadFile("./views/new.html")
	if err == nil {
		viewNew = string(b)
	}

	b, err = os.ReadFile("./views/view.html")
	if err == nil {
		viewView = string(b)
	}
}

func persistIndex() {
	b, _ := json.MarshalIndent(index, "", " ")
	os.WriteFile(dataDir+"/index_"+strconv.Itoa(siteConfig.IndexCount)+".json", b, 0644)
}
