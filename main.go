package main

import (
	"crypto/md5"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/3n3a/httpproxy-cache-api/modules/utils"
	hReq "github.com/imroc/req/v3"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bunrouter"
	"github.com/uptrace/bunrouter/extra/reqlog"
)

type Server struct {
	ProxyPath string
	Config map[string]string
	RedisClient utils.Redis
}

func (s *Server) Init() {
	s.ProxyPath = os.Getenv("APP_CONFIG_PATH")
	s.readConfig()

	port, err := strconv.ParseInt(s.getConfigValue("port").(string), 0, 0)
	if err != nil {
		panic("Error while parsing port from configuration")
	}
	
	router := bunrouter.New(
		bunrouter.Use(reqlog.NewMiddleware(
			reqlog.FromEnv("BUNDEBUG"),
		)),
		bunrouter.WithNotFoundHandler(notFoundHandler),
		bunrouter.WithMethodNotAllowedHandler(methodNotAllowedHandler),
	)
	
	s.RedisClient = utils.Redis{}
	s.RedisClient.Init(s.getConfigValue("redis-url").(string))
	
	router.GET("/v1/ping", s.pingHandler)
	
	router.GET("/v1/p/:key/*path", s.proxyHandler)
	router.POST("/v1/p/:key/*path", s.proxyHandler)
	router.PUT("/v1/p/:key/*path", s.proxyHandler)
	router.DELETE("/v1/p/:key/*path", s.proxyHandler)
	router.HEAD("/v1/p/:key/*path", s.proxyHandler)
	router.OPTIONS("/v1/p/:key/*path", s.proxyHandler)
	
	fmt.Printf("Listening on http://localhost:%d\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), router)
}

func main() {
	server := Server{}
	server.Init()
}

func (s *Server) readConfig() {
	if len(s.ProxyPath) <= 0 {
		panic("Proxy Configuration Path is empty")
	}

	var err error
	s.Config, err = utils.ReadYAMLIntoStruct[map[string]string](s.ProxyPath)
	if err != nil {
		panic("Configuration not found")
	}
}

func (s *Server) getConfigValue(key string) any {
	return s.Config[key]
}

func (s *Server) pingHandler (w http.ResponseWriter, req bunrouter.Request) error {
	value, err := s.RedisClient.Get("counter")
	if err == redis.Nil {
		s.RedisClient.Set("counter", 0, 5*time.Hour)
	}

	counter, _ := strconv.Atoi(value)
	counter++

	s.RedisClient.Set("counter", counter, 5*time.Hour)

	fmt.Println(counter)

	fmt.Fprintf(
		w,
		"pong",
	)
	return nil
}

func (s *Server) proxyHandler(w http.ResponseWriter, req bunrouter.Request) error {
	// Get Base Url for Key
	key := req.Param("key")
	path := req.Param("path")
	urlMap, err := utils.ReadYAMLIntoStruct[map[string]string](
		s.getConfigValue("proxy-path").(string),
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

	// Header Key
	headerKey := fmt.Sprintf("%s-header", bodyHash)

	cachedValue, err := s.RedisClient.Get(bodyHash)

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
		s.RedisClient.Set(bodyHash, encodedValue, 24*time.Hour)

		// Cache Headers
		jsonHeaders, err := json.Marshal(res.Response.Header)
		if err != nil {
			fmt.Println("Failed to marshal headers, Json.")
		}
		encodedHeaders := b64.StdEncoding.EncodeToString(jsonHeaders)
		s.RedisClient.Set(headerKey, encodedHeaders, 24*time.Hour)
	} else {
		// B64 Decode Body
		fmt.Println("Returning cached response")
		decodedValue, err := b64.StdEncoding.DecodeString(cachedValue)
		if err != nil {
			fmt.Println("Error decoding b64 value;", err)
		}

		// Get Headers from Cache
		cachedHeaders, err := s.RedisClient.Get(headerKey)
		if err != nil {
			fmt.Println("Failed to get header valzue")
		}

		// B64 Decode Headers
		jsonHeaders, err := b64.StdEncoding.DecodeString(cachedHeaders)
		if err != nil {
			fmt.Println("Failed to b564 decode headers")
		}

		// JSON Unmarshal Headers
		var decodedHeaders map[string][]string
		err = json.Unmarshal(jsonHeaders, &decodedHeaders)
		if err != nil {
			fmt.Println("Failed to unmarshal headers")
		}

		// Add Headers to HTTP Response
		for headerKey, headerValue := range decodedHeaders {
			w.Header().Add(headerKey, headerValue[0])
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
