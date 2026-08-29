// fund-hash-password — 生成 FUND_AUTH_PASSWORD_HASH 所需的 argon2id PHC 字符串。
// 用法：把输出直接写入部署 env 的 FUND_AUTH_PASSWORD_HASH（明文密码不会落盘/进 git）。
package main

import (
	"fmt"
	"os"

	"github.com/DeliciousBuding/fund-dashboard/internal/auth"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: fund-hash-password <password>")
		os.Exit(1)
	}
	hash, err := auth.HashPassword(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "hash failed:", err)
		os.Exit(1)
	}
	fmt.Println(hash)
}
