package sample

import "fmt"

// NoZap 无 zap 导入场景: 文件不包含 go.uber.org/zap 导入, 不应被修改
func NoZap() {
	fmt.Println("hello world")
	fmt.Printf("value: %d\n", 42)
}
