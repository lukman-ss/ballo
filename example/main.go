package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/lukman-ss/ballo"
)

func main() {
	e := ballo.New()

	// Global middleware
	e.Use(ballo.LoggerMiddleware())
	e.Use(ballo.RecoveryMiddleware())

	// Health check
	e.GET("/health", func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusOK, map[string]string{"status": "ok"}), nil
	})

	// API group
	api := e.Group("/api")
	api.GET("/users", func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusOK, []map[string]string{
			{"id": "1", "name": "Alice"},
			{"id": "2", "name": "Bob"},
		}), nil
	})
	api.GET("/users/:id", func(req *ballo.Request) (*ballo.Response, error) {
		id := req.Param("id")
		return ballo.JSON(http.StatusOK, map[string]string{"id": id, "name": "Alice"}), nil
	})
	api.POST("/users", func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusCreated, map[string]string{"message": "user created"}), nil
	})
	api.PUT("/users/:id", func(req *ballo.Request) (*ballo.Response, error) {
		id := req.Param("id")
		return ballo.JSON(http.StatusOK, map[string]string{"id": id, "message": "user updated"}), nil
	})
	api.DELETE("/users/:id", func(req *ballo.Request) (*ballo.Response, error) {
		id := req.Param("id")
		return ballo.JSON(http.StatusOK, map[string]string{"id": id, "message": "user deleted"}), nil
	})

	// 404 handler
	e.NotFound(func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusNotFound, map[string]string{"error": "not found"}), nil
	})

	// 405 handler
	e.MethodNotAllowed(func(req *ballo.Request) (*ballo.Response, error) {
		return ballo.JSON(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"}), nil
	})

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", e); err != nil {
		log.Fatal(err)
	}
}
