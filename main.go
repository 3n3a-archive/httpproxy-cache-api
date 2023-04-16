package main

import (
	"crypto/md5"
	b64 "encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/3n3a/httpproxy-cache-api/modules/utils"
	hReq "github.com/imroc/req/v3"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bunrouter"
	"github.com/uptrace/bunrouter/extra/reqlog"
)

const (
	PROXY_PATH = "./config/app.yaml"
)

var red utils.Redis

func main() {
	port, err := strconv.ParseInt(getConfigValue("port").(string), 0, 0)
	if err != nil {
		panic("No parsing of port possible")
	}

	router := bunrouter.New(
		bunrouter.Use(reqlog.NewMiddleware(
			reqlog.FromEnv("BUNDEBUG"),
		)),
		bunrouter.WithNotFoundHandler(notFoundHandler),
		bunrouter.WithMethodNotAllowedHandler(methodNotAllowedHandler),
	)

	red = utils.Redis{}
	red.Init()

	router.GET("/v1/ping", func(w http.ResponseWriter, req bunrouter.Request) error {
		value, err := red.Get("counter")
		if err == redis.Nil {
			red.Set("counter", 0, 5*time.Hour)
		}

		counter, _ := strconv.Atoi(value)
		counter++

		red.Set("counter", counter, 5*time.Hour)

		fmt.Println(counter)

		fmt.Fprintf(
			w,
			"pong",
		)
		return nil
	})

	router.GET("/v1/p/:key/*path", proxyHandler)
	router.POST("/v1/p/:key/*path", proxyHandler)
	router.PUT("/v1/p/:key/*path", proxyHandler)
	router.DELETE("/v1/p/:key/*path", proxyHandler)
	router.HEAD("/v1/p/:key/*path", proxyHandler)
	router.OPTIONS("/v1/p/:key/*path", proxyHandler)

	fmt.Printf("Listening on http://localhost:%d\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), router)
}

func getConfigValue(key string) any {
	config, err := utils.ReadYAMLIntoStruct[map[string]string](PROXY_PATH)
	if err != nil {
		panic("Configuration not found")
	}
	return config[key]
}

func proxyHandler(w http.ResponseWriter, req bunrouter.Request) error {
	// Get Base Url for Key
	key := req.Param("key")
	path := req.Param("path")
	urlMap, err := utils.ReadYAMLIntoStruct[map[string]string](
		getConfigValue("proxy-path").(string),
	)
	url := urlMap[key]
	if err != nil {
		fmt.Println("File not found")
	}
	builtUrl := fmt.Sprintf("%s/%s", url, path)

	// Read Input Body
	defer req.Body.Close()	
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		fmt.Println("Error reading req body")
	}

	// Hash the Body
	rawBodyHash := md5.Sum([]byte(reqBody))
	bodyHash := fmt.Sprintf("%x", rawBodyHash)

	cachedValue, err := red.Get(bodyHash)
	if err != nil {
		fmt.Println("Error getting cached value")
	}

	if err == redis.Nil {
		fmt.Println("Requesting from origin")
		// Proxy Request Headers & Body
		client := hReq.C()
		client.Headers = req.Header
		cReq := client.R()
		cReq.Method = req.Method
		cReq.SetURL(builtUrl)
		cReq.SetBody(reqBody)
		res := cReq.Do()
		err = res.Err
		if err != nil {
			fmt.Printf("Error Req: %s\n", err.Error())
		}

		// Proxy Response Headers
		for headerKey, headerValue := range res.Response.Header {
			w.Header().Add(headerKey, headerValue[0])
		}
	
		// Proxy Response Body
		w.Write(res.Bytes())

		// Cache Body
		encodedValue := b64.StdEncoding.EncodeToString(res.Bytes())
		red.Set(bodyHash, encodedValue, 24*time.Hour)
	} else {
		fmt.Println("Returning cached response")
		decodedValue, err := b64.StdEncoding.DecodeString(cachedValue)
		if err != nil {
			fmt.Println("Error decoding b64 value;", err)
		}
		w.Write(decodedValue)
	}


	return nil
}

func notFoundHandler(w http.ResponseWriter, req bunrouter.Request) error {
	return utils.JSON(
		w,
		bunrouter.H{
			"message": "route not found",
			"info": bunrouter.H{
				"path": req.URL.Path,
				"method": req.Method,
			},
		},
		http.StatusNotFound,
	)
}

func methodNotAllowedHandler(w http.ResponseWriter, req bunrouter.Request) error {
	return utils.JSON(
		w,
		bunrouter.H{
			"message": "route found, but method not allowed",
			"info": bunrouter.H{
				"path": req.URL.Path,
				"method": req.Method,
			},
		},
		http.StatusMethodNotAllowed,
	)
}
