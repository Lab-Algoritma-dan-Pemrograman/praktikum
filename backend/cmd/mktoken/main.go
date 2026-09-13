// Command mktoken membuat JWT uji dari JWT_SECRET di .env, tanpa perlu password.
// Hanya untuk pengujian lokal. Pakai: go run ./cmd/mktoken -uid 1150 -nim 202314020 -role asisten
package main

import (
	"flag"
	"fmt"
	"os"

	"lab-ap/config"
	"lab-ap/pkg/jwt"
)

func main() {
	uid := flag.Int("uid", 0, "users.id")
	nim := flag.String("nim", "", "NIM")
	role := flag.String("role", "mahasiswa", "role")
	flag.Parse()

	cfg := config.Load()
	tok, err := jwt.NewManager(cfg.JWTSecret, cfg.JWTExpireHours).Generate(*uid, *nim, *role)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
	fmt.Print(tok)
}
