// devtoken prints a test JWT. Only for local development.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	user := flag.String("user", "alice", "user ID")
	role := flag.String("role", "sizing_engineer", "viewer, sizing_engineer or admin")
	flag.Parse()

	claims := jwt.MapClaims{
		"sub":  *user,
		"role": *role,
		"exp":  time.Now().Add(1 * time.Hour).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(token)
}
