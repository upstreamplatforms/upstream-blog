package main

type Config struct {
	Title      string `json:"title"`
	IndexSize  int    `json:"indexSize"`
	IndexCount int    `json:"indexCount"`
}

type Index struct {
	Posts []PostMeta `json:"posts"`
}

type PostMeta struct {
	Id       string   `json:"id"`
	Title    string   `json:"title"`
	Excerpt  string   `json:"excerpt"`
	Category string   `json:"category"`
	Date     int64    `json:"date"`
	ReadTime string   `json:"readTime"`
	Image    string   `json:"image"`
	Tags     []string `json:"tags"`
}

type Post struct {
	Id       string   `json:"id"`
	Title    string   `json:"title"`
	Excerpt  string   `json:"excerpt"`
	Category string   `json:"category"`
	Date     int64    `json:"date"`
	ReadTime string   `json:"readTime"`
	Image    string   `json:"image"`
	Tags     []string `json:"tags"`
	Content  string   `json:"content"`
}
