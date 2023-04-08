package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/3n3a/httpproxy-cache-api/modules/utils"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bunrouter"
	"github.com/uptrace/bunrouter/extra/reqlog"
)

func main() {
	port := 5001
	router := bunrouter.New(
		bunrouter.Use(reqlog.NewMiddleware(
			reqlog.FromEnv("BUNDEBUG"),
		)),
		bunrouter.WithNotFoundHandler(notFoundHandler),
		bunrouter.WithMethodNotAllowedHandler(methodNotAllowedHandler),
	)

	r := utils.Redis{}
	r.Init()

	router.GET("/v1/ping", func(w http.ResponseWriter, req bunrouter.Request) error {
		value, err := r.Get("counter")
		if err == redis.Nil {
			r.Set("counter", 0, 5*time.Hour)
		}

		counter, _ := strconv.Atoi(value)
		counter++

		r.Set("counter", counter, 5*time.Hour)

		fmt.Println(counter)

		fmt.Fprintf(
			w,
			"pong",
		)
		return nil
	})


	fmt.Printf("Listening on http://localhost:%d\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), router)
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
