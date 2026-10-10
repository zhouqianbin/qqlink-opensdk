module github.com/zhouqianbin/qqlink-opensdk

// 只依赖标准库。版本要求放宽到 1.18（泛型可用），
// 以便接入方不必升级工具链——SDK 自身未使用更高版本特性。
go 1.18
