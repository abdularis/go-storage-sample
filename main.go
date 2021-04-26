package main

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/abdularis/go-storage"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const secret = "this-is-your-hmac-secret"

func generateHmac(expireAt string, requestURI string) (string, error) {
	data, err := json.Marshal(map[string]interface{}{
		"expireAt":   expireAt,
		"requestURI": requestURI,
	})
	if err != nil {
		return "", err
	}

	h := hmac.New(sha512.New, []byte(secret))
	h.Write(data)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil)), nil
}

func signedURLMiddleware(context *gin.Context) {
	expireAt := context.Query("expireAt")
	expire, err := strconv.ParseInt(expireAt, 10, 64)
	if err != nil {
		context.AbortWithStatusJSON(400, "Invalid expiration arg")
		return
	}

	if time.Now().Unix() > expire {
		context.AbortWithStatusJSON(400, "URL expired")
		return
	}

	u := context.Request.URL
	q := u.Query()

	signature := q.Get("signature")
	q.Del("signature")
	u.RawQuery = q.Encode()

	generatedSignature, err := generateHmac(expireAt, u.RequestURI())

	if generatedSignature != signature {
		context.AbortWithStatusJSON(403, "No Access Sorry Bre 🤚 ⛔")
		return
	}
}

func signedURLBuilder(absoluteFilePath string, objectPath string, expireIn time.Duration) (string, error) {
	u, err := url.Parse("http://localhost:8000/private/files")
	if err != nil {
		return "", err
	}

	expireAt := fmt.Sprintf("%d", time.Now().Add(expireIn).Unix())

	q := u.Query()
	q.Add("expireAt", expireAt)

	u.Path = path.Join(u.Path, objectPath)
	u.RawQuery = q.Encode()

	signature, err := generateHmac(expireAt, u.RequestURI())
	if err != nil {
		return "", err
	}

	q.Add("signature", signature)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func main() {
	c := gin.Default()

	// Serve public files
	c.StaticFS("/files", http.Dir("storage/public"))

	// Serve private files
	c.Use(signedURLMiddleware).StaticFS("/private/files", http.Dir("storage/private"))

	storage := gostorage.NewLocalStorage(
		"storage/private",
		"storage/public",
		"http://localhost:8000/files",
		signedURLBuilder)

	dataSource := strings.NewReader("Hello, this is content 😊 😅 updated")
	err := storage.Put("user-files/sample.txt", dataSource, gostorage.ObjectPublicRead)
	if err != nil {
		panic(err)
	}

	u, err := storage.URL("user-files/sample.txt")
	if err != nil {
		panic(err)
	}

	fmt.Printf("url = %s\n", u)

	u, err = storage.TemporaryURL("user-files/sample.txt", time.Minute)
	if err != nil {
		panic(err)
	}

	fmt.Printf("signed url = %s\n", u)

	_ = c.Run("localhost:8000")
}
