# Web Terminal

基于浏览器的 Windows 终端模拟器，使用 Go 和 [xterm.js](https://xtermjs.org/) 构建。

通过 Web 界面登录后，即可在浏览器中访问完整的 Windows 终端（cmd.exe、PowerShell 等）。

## 功能特性

- 基于 xterm.js 的浏览器终端模拟
- 用户名/密码认证
- 基于 IP 的防暴力破解（可配置最大尝试次数和封禁时长）
- 工作目录选择与历史记录
- 终端窗口大小自适应
- WebSocket 实时通信
- 静态资源内嵌二进制，单文件部署

## 快速开始

### 环境要求

- Go 1.25+
- Windows（使用 Windows ConPty API）
- GCC 工具链（go-sqlite3 依赖，如 [MinGW-w64](https://www.mingw-w64.org/)）

### 编译运行

```bash
go build -o web-terminal.exe
./web-terminal.exe
```

浏览器打开 http://localhost:8080 即可使用。

### 自定义配置文件

```bash
./web-terminal.exe -config /path/to/config.yaml
```

## 配置说明

在工作目录下创建 `config.yaml`：

```yaml
server:
  port: 8080

auth:
  username: admin
  password: changeme        # 请修改密码
  max_attempts: 5           # 登录失败次数上限
  block_duration: 30m       # IP 封禁时长

terminal:
  shell: cmd.exe            # cmd.exe、powershell.exe 等
```

所有字段均有默认值，可选配置。

## 项目结构

```
web-terminal/
├── main.go                  # 程序入口
├── config.yaml              # 配置文件
├── internal/
│   ├── config/              # 配置加载
│   ├── auth/                # 认证、会话令牌、防暴力破解
│   ├── server/              # HTTP/WebSocket 服务
│   └── terminal/            # Windows ConPty 终端封装
└── web/
    ├── index.html           # 前端页面
    ├── terminal.js          # 终端逻辑
    └── style.css            # 样式
```

## 工作流程

1. 用户打开网页，输入账号密码登录
2. 认证通过后，服务端签发会话令牌
3. 用户选择工作目录（支持历史记录）
4. 建立 WebSocket 连接，通过 Windows ConPty 启动 Shell 进程
5. 终端输入输出通过 WebSocket 实时传输

## API

| 接口 | 方法 | 说明 |
|---|---|---|
| `/api/login` | POST | 登录认证，获取会话令牌 |
| `/ws/terminal` | WebSocket | 终端 I/O 流（需令牌） |
| `/api/dirs` | GET | 获取目录历史（需令牌） |
| `/api/dirs` | POST | 记录目录访问（需令牌） |

## 快捷键

- `Ctrl+Shift+L` — 注销登录

## 许可证

MIT
