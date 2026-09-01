// fund-hash-password — 生成 FUND_AUTH_PASSWORD_HASH 所需的 argon2id PHC 字符串。
//
// 用法（把输出直接写入部署 env 的 FUND_AUTH_PASSWORD_HASH；哈希不暴露明文）：
//
//	FUND_HASH_PASSWORD_INPUT='<password>' fund-hash-password   # 推荐：不经进程列表
//	echo '<password>' | fund-hash-password                     # stdin：末尾换行会被去掉
//	fund-hash-password '<password>'                            # 旧用法兼容：明文会短暂出现在
//	                                                           # 进程列表/shell 历史，运行时会打告警
//
// 读取优先级：FUND_HASH_PASSWORD_INPUT 环境变量 → stdin → argv。argv 仅作为
// 最后兼容手段保留，并输出告警。
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DeliciousBuding/fund-dashboard/internal/auth"
)

const passwordEnvVar = "FUND_HASH_PASSWORD_INPUT"

// passwordInput 携带明文及其来源，argv 来源用于在 main 输出告警。
type passwordInput struct {
	value  string
	source string
}

const (
	sourceEnv   = "env"
	sourceStdin = "stdin"
	sourceArgv  = "argv"
)

func main() {
	os.Exit(run(os.Stdout, os.Stderr, os.Args[1:], os.LookupEnv, os.Stdin))
}

// run 与 main 分离以便测试；返回进程退出码。
func run(stdout, stderr io.Writer, args []string, getenv func(string) (string, bool), stdin io.Reader) int {
	input, err := readPasswordInput(args, getenv, stdin)
	if err != nil {
		fmt.Fprintln(stderr, "fund-hash-password: "+err.Error())
		fmt.Fprintf(stderr, "usage: fund-hash-password [<password>]  # 或设置 %s / 从 stdin 传入\n", passwordEnvVar)
		return 1
	}
	if input.source == sourceArgv {
		fmt.Fprintf(stderr, "fund-hash-password: warning: argv 中的密码会暴露在进程列表/shell 历史；建议改用 %s 环境变量或 stdin\n", passwordEnvVar)
	}
	hash, err := auth.HashPassword(input.value)
	if err != nil {
		fmt.Fprintln(stderr, "hash failed:", err)
		return 1
	}
	fmt.Fprintln(stdout, hash)
	return 0
}

// readPasswordInput 按优先级解析明文密码：环境变量 → stdin → argv。
// 显式空值（空环境变量 / 空 stdin / 空 argv）一律报错，避免静默散列空密码。
func readPasswordInput(args []string, getenv func(string) (string, bool), stdin io.Reader) (passwordInput, error) {
	if v, ok := getenv(passwordEnvVar); ok {
		if v == "" {
			return passwordInput{}, fmt.Errorf("%s 已设置但为空", passwordEnvVar)
		}
		return passwordInput{value: v, source: sourceEnv}, nil
	}

	switch len(args) {
	case 0:
		data, err := io.ReadAll(stdin)
		if err != nil {
			return passwordInput{}, fmt.Errorf("read stdin: %w", err)
		}
		// 去掉 echo/管道带来的尾部换行；其余前后空白按密码内容原样保留。
		value := strings.TrimRight(string(data), "\r\n")
		if value == "" {
			return passwordInput{}, errors.New("stdin 中的密码为空")
		}
		return passwordInput{value: value, source: sourceStdin}, nil
	case 1:
		if args[0] == "" {
			return passwordInput{}, errors.New("argv 中的密码为空")
		}
		return passwordInput{value: args[0], source: sourceArgv}, nil
	default:
		return passwordInput{}, errors.New("至多接收一个密码参数")
	}
}
