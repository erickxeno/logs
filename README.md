[![codecov](https://codecov.io/gh/erickxeno/logs/branch/master/graph/badge.svg)](https://codecov.io/gh/erickxeno/logs)

# Xeno Logs

## English Version

A high-performance, feature-rich logging library for Go applications.

### Features

- **Structured Logging**: Support for key-value pairs and structured data
- **Multiple Log Levels**: Trace, Debug, Info, Notice, Warn, Error, Fatal
- **Context Support**: Built-in context integration for request tracing and metadata
- **Flexible Output**: Support for console, file, and custom writers
- **Performance Optimized**: 
  - Zero allocation for common operations
  - Object pooling for log instances
  - Efficient buffer management
- **Customizable Format**:
  - Configurable file path depth
  - Optional function name display
  - Customizable time format with timezone support
- **Middleware Support**: Extensible middleware system for log processing
- **Rate Limiting**: Built-in rate limiting for log output
- **Compatibility Mode**: Support for traditional logging style

### Quick Start

```go
package main

import (
    "github.com/erickxeno/logs"
)

func main() {
    // Create a new logger with default settings
    logger := logs.NewCLogger(
        logs.SetPSM("my-service"),
        logs.SetFileDepth(1),  // Show last directory + filename
    )

    // Basic logging
    logger.Info("Hello, World!")

    // Logging with key-value pairs
    logger.Info("User logged in").KV("user_id", 123).KV("ip", "127.0.0.1")

    // Context-based logging
    ctx := context.Background()
    logger.Info(ctx, "Request processed").KV("request_id", "abc-123")

    // Error logging with stack trace
    if err != nil {
        logger.Error("Operation failed").Err(err)
    }
}
```

### Advanced Usage

#### Custom Writer

```go
// Create a custom writer
writer := logs.NewConsoleWriter(
    logs.SetColorful(true),
)

// Create logger with custom writer
logger := logs.NewCLogger(
    logs.SetWriter(logs.InfoLevel, writer),
)
```

#### Middleware

```go
// Create a middleware that adds request ID to all logs
middleware := func(next logs.Middleware) logs.Middleware {
    return func(l *logs.Log) *logs.Log {
        return l.KV("request_id", uuid.New())
    }
}

logger := logs.NewCLogger(
    logs.SetMiddleware(middleware),
)
```

#### Rate Limiting

```go
// Create a rate-limited logger
logger := logs.NewCLogger(
    logs.SetRateLimit(100), // 100 logs per second
)
```

### Configuration Options

- `SetPSM`: Set service name
- `SetFileDepth`: Control file path display depth
- `SetFullPath`: Show full file path
- `SetDisplayFuncName`: Show function name in logs
- `SetCallDepth`: Adjust stack trace depth
- `SetPadding`: Customize log field separator
- `SetConvertErrorToKV`: Convert errors to key-value pairs
- `SetConvertObjectToKV`: Convert objects to key-value pairs

### Performance

The library is optimized for performance with zero allocation for common operations and efficient buffer management.

### Performance Comparison with Go Standard Library

#### Test Environment
```
goos: darwin
goarch: amd64
pkg: github.com/erickxeno/logs
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
```

#### Detailed Benchmark Results
```
BenchmarkXenoVsStdLog/Xeno_Simple-12            91878649                15.26 ns/op            0 B/op          0 allocs/op
BenchmarkXenoVsStdLog/Std_Simple-12             29200424                37.28 ns/op           16 B/op          1 allocs/op
BenchmarkXenoVsStdLog/Xeno_WithKV-12             5445943               233.4 ns/op            48 B/op          3 allocs/op
BenchmarkXenoVsStdLog/Std_WithKV-12             21164678                53.87 ns/op           16 B/op          1 allocs/op
BenchmarkXenoVsStdLog/Xeno_WithContext-12        5175687               224.1 ns/op            48 B/op          3 allocs/op
BenchmarkXenoVsStdLog/Xeno_WithError-12         81696086                16.23 ns/op            0 B/op          0 allocs/op
BenchmarkXenoVsStdLog/Std_WithError-12          31007655                43.95 ns/op           16 B/op          1 allocs/op
BenchmarkXenoVsStdLog/Xeno_Concurrent-12        21555132                79.50 ns/op           48 B/op          3 allocs/op
BenchmarkXenoVsStdLog/Std_Concurrent-12         56980738                17.87 ns/op           16 B/op          1 allocs/op
```

#### Simple Logging
- Xeno Logs: 15.26 ns/op, 0 B/op, 0 allocs/op
- Standard Library: 37.28 ns/op, 16 B/op, 1 allocs/op
Xeno Logs performs better in simple logging scenarios with no memory allocation.

#### Logging with Key-Value Pairs
- Xeno Logs: 233.4 ns/op, 48 B/op, 3 allocs/op
- Standard Library: 53.87 ns/op, 16 B/op, 1 allocs/op
The standard library performs better in key-value logging scenarios with less memory allocation.

#### Context-based Logging
- Xeno Logs: 224.1 ns/op, 48 B/op, 3 allocs/op
- Standard Library: Not tested
Xeno Logs provides context support with additional performance overhead.

#### Error Logging
- Xeno Logs: 16.23 ns/op, 0 B/op, 0 allocs/op
- Standard Library: 43.95 ns/op, 16 B/op, 1 allocs/op
Xeno Logs performs better in error logging scenarios with no memory allocation.

#### Concurrent Logging
- Xeno Logs: 79.50 ns/op, 48 B/op, 3 allocs/op
- Standard Library: 17.87 ns/op, 16 B/op, 1 allocs/op
The standard library performs better in concurrent scenarios with less memory allocation.

#### Summary
1. Xeno Logs performs better in simple logging and error logging scenarios with zero memory allocation
2. The standard library performs better in key-value and concurrent logging scenarios
3. Xeno Logs requires more memory allocation in complex scenarios (key-value pairs, context)
4. The standard library maintains consistent memory allocation (16 bytes) across all scenarios

#### Recommendations
1. Use Xeno Logs if your primary use case is simple logging and error logging
2. Consider using the standard library if you heavily use key-value logging or concurrent logging
3. The standard library might be more suitable if memory allocation is a critical factor
4. Xeno Logs is the better choice if you need advanced features like context support

### License

This project is licensed under the MIT License - see the LICENSE file for details.

---

## 中文版本

高性能、功能丰富的 Go 日志库。

### 特性

- **结构化日志**: 支持键值对和结构化数据
- **多级别日志**: Trace, Debug, Info, Notice, Warn, Error, Fatal
- **上下文支持**: 内置请求追踪和元数据集成
- **灵活输出**: 支持控制台、文件和自定义写入器
- **性能优化**: 
  - 常见操作零内存分配
  - 日志实例对象池
  - 高效缓冲区管理
- **可定制格式**:
  - 可配置文件路径深度
  - 可选函数名显示
  - 可自定义时间格式（支持时区）
- **中间件支持**: 可扩展的日志处理中间件系统
- **速率限制**: 内置日志输出速率限制
- **兼容模式**: 支持传统日志风格

### 快速开始

```go
package main

import (
    "github.com/erickxeno/logs"
)

func main() {
    // 创建带有默认设置的日志器
    logger := logs.NewCLogger(
        logs.SetPSM("my-service"),
        logs.SetFileDepth(1),  // 显示最后一级目录和文件名
    )

    // 基础日志记录
    logger.Info("Hello, World!")

    // 带键值对的日志记录
    logger.Info("用户登录").KV("user_id", 123).KV("ip", "127.0.0.1")

    // 基于上下文的日志记录
    ctx := context.Background()
    logger.Info(ctx, "请求处理完成").KV("request_id", "abc-123")

    // 带堆栈跟踪的错误日志
    if err != nil {
        logger.Error("操作失败").Err(err)
    }
}
```

### 高级用法

#### 自定义写入器

```go
// 创建自定义写入器
writer := logs.NewConsoleWriter(
    logs.SetColorful(true),
)

// 使用自定义写入器创建日志器
logger := logs.NewCLogger(
    logs.SetWriter(logs.InfoLevel, writer),
)
```

#### 中间件

```go
// 创建添加请求ID的中间件
middleware := func(next logs.Middleware) logs.Middleware {
    return func(l *logs.Log) *logs.Log {
        return l.KV("request_id", uuid.New())
    }
}

logger := logs.NewCLogger(
    logs.SetMiddleware(middleware),
)
```

#### 速率限制

```go
// 创建带速率限制的日志器
logger := logs.NewCLogger(
    logs.SetRateLimit(100), // 每秒100条日志
)
```

### 配置选项

- `SetPSM`: 设置服务名称
- `SetFileDepth`: 控制文件路径显示深度
- `SetFullPath`: 显示完整文件路径
- `SetDisplayFuncName`: 在日志中显示函数名
- `SetCallDepth`: 调整堆栈跟踪深度
- `SetPadding`: 自定义日志字段分隔符
- `SetConvertErrorToKV`: 将错误转换为键值对
- `SetConvertObjectToKV`: 将对象转换为键值对

### 性能

该库针对性能进行了优化，常见操作零分配，并具有高效的缓冲区管理。

### 与 Go 标准库的性能对比

#### 测试环境
```
goos: darwin
goarch: amd64
pkg: github.com/erickxeno/logs
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
```

#### 详细性能测试结果
```
BenchmarkXenoVsStdLog/Xeno_Simple-12            91878649                15.26 ns/op            0 B/op          0 allocs/op
BenchmarkXenoVsStdLog/Std_Simple-12             29200424                37.28 ns/op           16 B/op          1 allocs/op
BenchmarkXenoVsStdLog/Xeno_WithKV-12             5445943               233.4 ns/op            48 B/op          3 allocs/op
BenchmarkXenoVsStdLog/Std_WithKV-12             21164678                53.87 ns/op           16 B/op          1 allocs/op
BenchmarkXenoVsStdLog/Xeno_WithContext-12        5175687               224.1 ns/op            48 B/op          3 allocs/op
BenchmarkXenoVsStdLog/Xeno_WithError-12         81696086                16.23 ns/op            0 B/op          0 allocs/op
BenchmarkXenoVsStdLog/Std_WithError-12          31007655                43.95 ns/op           16 B/op          1 allocs/op
BenchmarkXenoVsStdLog/Xeno_Concurrent-12        21555132                79.50 ns/op           48 B/op          3 allocs/op
BenchmarkXenoVsStdLog/Std_Concurrent-12         56980738                17.87 ns/op           16 B/op          1 allocs/op
```

#### 简单日志记录
- Xeno Logs: 15.26 ns/op, 0 B/op, 0 allocs/op
- 标准库: 37.28 ns/op, 16 B/op, 1 allocs/op
Xeno Logs 在简单日志场景下性能更好，且没有内存分配。

#### 带键值对的日志
- Xeno Logs: 233.4 ns/op, 48 B/op, 3 allocs/op
- 标准库: 53.87 ns/op, 16 B/op, 1 allocs/op
标准库在带键值对的场景下性能更好，内存分配也更少。

#### 带上下文的日志
- Xeno Logs: 224.1 ns/op, 48 B/op, 3 allocs/op
- 标准库: 未测试
Xeno Logs 提供了上下文支持，但需要额外的性能开销。

#### 错误日志
- Xeno Logs: 16.23 ns/op, 0 B/op, 0 allocs/op
- 标准库: 43.95 ns/op, 16 B/op, 1 allocs/op
Xeno Logs 在错误日志场景下性能更好，且没有内存分配。

#### 并发日志
- Xeno Logs: 79.50 ns/op, 48 B/op, 3 allocs/op
- 标准库: 17.87 ns/op, 16 B/op, 1 allocs/op
标准库在并发场景下性能更好，内存分配也更少。

#### 总结
1. Xeno Logs 在简单日志和错误日志场景下表现更好，且没有内存分配
2. 标准库在带键值对和并发场景下表现更好
3. Xeno Logs 在复杂场景（如带键值对、上下文）下需要更多的内存分配
4. 标准库在所有场景下都保持稳定的内存分配（16字节）

#### 建议
1. 如果主要使用简单日志和错误日志，推荐使用 Xeno Logs
2. 如果需要大量使用键值对日志或并发日志，可以考虑使用标准库
3. 如果内存分配是关键考虑因素，标准库可能更适合
4. 如果需要上下文支持等高级特性，Xeno Logs 是更好的选择

### 许可证

本项目采用 MIT 许可证 - 详见 LICENSE 文件。  
