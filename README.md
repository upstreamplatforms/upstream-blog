# Upstream Blog

## Creating posts
```sh
# create one post
curl -X POST http://0:8080/posts -d '{"title": "Test post...", "content": "test content..."}'

# create 50 posts
for i in {1..50}
do
  curl -X POST http://0:8080/posts -d "{\"title\": \"Test post $i\", \"content\": \"test content $i...\"}"
done
```
