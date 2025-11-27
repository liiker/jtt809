package jtt809

import (
	"io"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// EncodeGBK 将 UTF-8 字符串转换为 GBK 字节切片。
func EncodeGBK(s string) ([]byte, error) {
	reader := transform.NewReader(strings.NewReader(s), simplifiedchinese.GBK.NewEncoder())
	d, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// DecodeGBK 将 GBK 字节切片转换为 UTF-8 字符串，并去除尾部 \x00。
func DecodeGBK(src []byte) (string, error) {
	decoder := simplifiedchinese.GBK.NewDecoder()
	s, _, err := transform.String(decoder, string(src))
	if err != nil {
		return string(src), err
	}
	return strings.TrimRight(s, "\x00"), nil
}

// PadRightGBK 将字符串转为 GBK 后，右侧补零至指定长度。
// 如果转换后长度超过 length，则截断。
func PadRightGBK(s string, length int) []byte {
	gbkBytes, _ := EncodeGBK(s) // 忽略错误，降级为空或部分数据
	if len(gbkBytes) >= length {
		return gbkBytes[:length]
	}
	padding := make([]byte, length-len(gbkBytes))
	return append(gbkBytes, padding...)
}

// PadLeftGBK 将字符串转为 GBK 后，左侧补零至指定长度。
// 如果转换后长度超过 length，则截断（保留右侧）。
func PadLeftGBK(s string, length int) []byte {
	gbkBytes, _ := EncodeGBK(s)
	if len(gbkBytes) >= length {
		return gbkBytes[len(gbkBytes)-length:]
	}
	padding := make([]byte, length-len(gbkBytes))
	return append(padding, gbkBytes...)
}
