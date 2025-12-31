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

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

var siteConfig Config
var index Index
var viewIndex []byte
var viewNew []byte
var viewView []byte
var staticCache map[string]string = make(map[string]string)
var postCache map[string][]byte = make(map[string][]byte)
var dataDir = "./data"
var editorsList []string
var jwtCerts = ""
var projectId = ""
var monthViewsName = time.Now().Format("200601") + ".json"
var monthViews map[string][]int64

func main() {

	Init()

	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-signalChannel
		switch sig {
		case os.Interrupt:
			fmt.Println("Interrupt received, persisting index and closing.")
			b, _ := json.MarshalIndent(monthViews, "", " ")
			os.WriteFile(dataDir+"/analytics/"+monthViewsName, b, 0644)
			os.Exit(1)
		case syscall.SIGKILL:
			fmt.Println("SIGINT received, persisting index and closing.")
			os.Exit(1)
		case syscall.SIGTERM:
			fmt.Println("SIGTERM received, persisting index and closing.")
			b, _ := json.MarshalIndent(monthViews, "", " ")
			os.WriteFile(dataDir+"/analytics/"+monthViewsName, b, 0644)
			os.Exit(1)
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Write(viewIndex)
	})

	mux.HandleFunc("GET /config", func(w http.ResponseWriter, r *http.Request) {
		config := map[string]string{
			"authApiKey": os.Getenv("AUTH_API_KEY"),
			"authDomain": os.Getenv("AUTH_DOMAIN"),
		}
		b, _ := json.MarshalIndent(config, "", " ")
		fmt.Fprint(w, string(b))
	})

	mux.HandleFunc("GET /users/{email}", func(w http.ResponseWriter, r *http.Request) {
		email := r.PathValue("email")
		isEditor := slices.Contains(editorsList, email)
		userData := User{Email: email, Roles: []string{"Reader"}}

		if isEditor {
			userData.Roles = append(userData.Roles, "Editor")
		}

		b, _ := json.MarshalIndent(userData, "", " ")
		fmt.Fprint(w, string(b))
	})

	mux.HandleFunc("GET /new", func(w http.ResponseWriter, r *http.Request) {
		w.Write(viewNew)
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
		if strings.HasPrefix(r.URL.Path, "/20") {
			w.Write(viewView)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	mux.HandleFunc("GET /edit", func(w http.ResponseWriter, r *http.Request) {
		w.Write(viewNew)
	})

	mux.HandleFunc("GET /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		b, ok := postCache[id]
		if !ok {
			b, _ = os.ReadFile(dataDir + "/posts/" + id + ".json")
			if b != nil {
				postCache[id] = b
			}
		}

		if b != nil {
			w.Write(b)

			analyticsData, ok := monthViews[id]
			if ok {
				analyticsData = append(analyticsData, time.Now().Unix())
				monthViews[id] = analyticsData
			} else {
				monthViews[id] = []int64{time.Now().Unix()}
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	mux.HandleFunc("POST /posts", func(w http.ResponseWriter, r *http.Request) {
		// validate id token
		idToken := r.Header["Authorization"][0]
		idToken = idToken[7:]

		k, jwksErr := keyfunc.NewJWKSetJSON(json.RawMessage(jwtCerts))
		if jwksErr != nil {
			fmt.Println(jwksErr.Error())
		}
		token, _ := jwt.Parse(idToken, k.Keyfunc, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
		email := ""
		iss := ""
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			email = claims["email"].(string)
			iss = claims["iss"].(string)
		}
		isEditor := slices.Contains(editorsList, email)
		isProject := strings.HasSuffix(iss, "/"+projectId)
		if !isEditor || !isProject {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "Not authorized.")
			return
		}

		// download an image file
		imageResponse, _ := http.Get("https://picsum.photos/200/200")
		var fileName string
		if imageResponse != nil {
			fileName = RandomString(5)
			imageFile, _ := os.Create(dataDir + "/images/" + fileName + ".jpg")
			defer imageResponse.Body.Close()
			io.Copy(imageFile, imageResponse.Body)
		}

		var newContent []byte
		var err error
		newContent, err = io.ReadAll(r.Body)
		if err != nil {
			log.Println(err.Error())
		}

		var newPost Post
		json.Unmarshal(newContent, &newPost)
		lastIdPiece := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(newPost.Title), "&", "and"), "?", "")
		lastIdPiece = strings.ReplaceAll(lastIdPiece, "\"", "")
		lastIdPiece = strings.ReplaceAll(lastIdPiece, " ", "-")
		if len(lastIdPiece) > 27 {
			lastIdPiece = lastIdPiece[0:27]
		}
		newPost.Id = time.Now().Format("20060102-150405-") + lastIdPiece
		newPost.Date = time.Now().Unix()
		newPost.Page = siteConfig.IndexCount
		if newPost.Category == "" {
			newPost.Category = "General"
		}

		wordCount := strings.Split(newPost.Content, " ")
		readMinutes := len(wordCount) / 200
		readMinutes = max(1, readMinutes)
		newPost.ReadTime = strconv.Itoa(readMinutes) + " min"
		newPost.Image = "/images/" + fileName + ".jpg" // "https://picsum.photos/200/300" // "linear-gradient(135deg, #fce7f3 0%, #fbcfe8 100%)"
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
			index.Posts = slices.Delete(index.Posts, 0, siteConfig.IndexSize)
			siteConfig.IndexCount++

			b, _ = json.MarshalIndent(siteConfig, "", " ")
			os.WriteFile(dataDir+"/config.json", b, 0644)
		}

		go persistIndex()

		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, "OK")
	})

	mux.HandleFunc("PUT /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		// validate id token
		idToken := r.Header["Authorization"][0]
		idToken = idToken[7:]

		k, jwksErr := keyfunc.NewJWKSetJSON(json.RawMessage(jwtCerts))
		if jwksErr != nil {
			fmt.Println(jwksErr.Error())
		}
		token, _ := jwt.Parse(idToken, k.Keyfunc, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
		email := ""
		iss := ""
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			email = claims["email"].(string)
			iss = claims["iss"].(string)
		}
		isEditor := slices.Contains(editorsList, email)
		isProject := strings.HasSuffix(iss, "/"+projectId)
		if !isEditor || !isProject {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "Not authorized.")
			return
		}

		var newContent []byte
		var err error
		newContent, err = io.ReadAll(r.Body)
		if err != nil {
			log.Println(err.Error())
		}

		var newPost Post
		json.Unmarshal(newContent, &newPost)

		wordCount := strings.Split(newPost.Content, " ")
		readMinutes := len(wordCount) / 200
		readMinutes = max(1, readMinutes)
		newPost.ReadTime = strconv.Itoa(readMinutes) + " min"
		postBytes, _ := json.MarshalIndent(newPost, "", "  ")
		os.WriteFile(dataDir+"/posts/"+newPost.Id+".json", postBytes, 0644)
		postCache[newPost.Id] = postBytes

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

	mux.HandleFunc("GET /images/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		b, err := os.ReadFile(dataDir + "/images/" + name)
		if err == nil {
			w.Header().Set("Content-Type", "image/jpg")
			w.WriteHeader(http.StatusOK)
			w.Write(b)
		}
	})

	log.Print("Listening...")

	http.ListenAndServe(":8080", mux)
}

func Init() {
	start := time.Now()

	tempDataDir := os.Getenv("DATA_DIR")
	if tempDataDir != "" {
		dataDir = tempDataDir
	}
	os.MkdirAll(dataDir+"/posts", os.ModePerm)
	os.MkdirAll(dataDir+"/images", os.ModePerm)
	os.MkdirAll(dataDir+"/analytics", os.ModePerm)

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

	// load viewer data
	b, err = os.ReadFile(dataDir + "/analytics/" + monthViewsName)
	if err == nil {
		json.Unmarshal(b, &monthViews)
	} else {
		monthViews = make(map[string][]int64)
	}

	// load views
	b, err = os.ReadFile("./views/index.html")
	if err == nil {
		viewIndex = b
	}

	b, err = os.ReadFile("./views/new.html")
	if err == nil {
		viewNew = b
	}

	b, err = os.ReadFile("./views/view.html")
	if err == nil {
		viewView = b
	}

	// load project
	projectId = os.Getenv("GCLOUD_PROJECT")

	// load editors
	editors := os.Getenv("EDITORS")
	editorsList = strings.Split(editors, ",")

	elapsed := time.Since(start)
	log.Printf("Init took %s", elapsed)

	// load certs
	go loadCerts()
}

func loadCerts() {
	certResponse, _ := http.Get("https://www.googleapis.com/oauth2/v3/certs")
	if certResponse != nil {
		body, _ := io.ReadAll(certResponse.Body)
		if body != nil {
			jwtCerts = string(body)
		}
	}
}

func persistIndex() {
	b, _ := json.MarshalIndent(index, "", " ")
	os.WriteFile(dataDir+"/index_"+strconv.Itoa(siteConfig.IndexCount)+".json", b, 0644)
}
