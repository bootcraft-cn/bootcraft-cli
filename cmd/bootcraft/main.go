package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/bootcraft-cn/cli/internal/commands"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "login":
		err = commands.LoginCommand(os.Args[2:])
	case "submit":
		err = commands.SubmitCommand(os.Args[2:])
	case "version", "--version", "-v":
		commands.VersionCommand()
		return
	case "help", "--help", "-h":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "错误: %s\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`bootcraft — Bootcraft 平台 CLI 工具

用法:
  bootcraft <命令> [选项]

命令:
  login     登录 Bootcraft 平台
  submit    提交代码进行评测
  version   显示版本信息

选项:
  --help    显示帮助信息

示例:
  bootcraft login                     浏览器授权登录
  bootcraft login --token bc_xxx      使用 Token 登录
  bootcraft submit                    提交当前目录代码
  bootcraft submit --stage softmax     指定评测关卡
  bootcraft submit --dry-run          仅预览打包内容`)
}
