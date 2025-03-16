module voiceworld-go-sdk

go 1.23.0

toolchain go1.24.1

// 使用本地包替换远程包
replace github.com/voiceworld/voiceworld-go-sdk => ./

require github.com/aliyun/alibabacloud-oss-go-sdk-v2 v1.2.1

require golang.org/x/time v0.4.0 // indirect
