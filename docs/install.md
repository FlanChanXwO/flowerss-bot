# 部署

## 二进制部署

从 [Releases](https://github.com/indes/flowerss-bot/releases) 页面下载对应的版本解压运行即可。

## Docker 部署

无需在服务器安装 Go 或构建 Docker 镜像。GitHub Actions 会自动将 amd64/arm64 镜像发布到 GHCR。

如果已经部署了旧版，请先停止容器并备份 `data.db`，再拉取新镜像并重建容器。新版本会在启动时为现有订阅表自动增加话题字段，不会删除原订阅。

### Telegram 话题群组

把机器人加入已开启“话题”的超级群组后，在**目标话题内**执行：

- `/sub <RSS URL>`：只订阅到当前话题，RSS 的新消息和多图相册都推送到当前话题。
- `/list`、`/unsub`、`/set`、`/unsuball`：只处理当前话题的 RSS 订阅。
- `/pauseall`、`/activeall`：仅暂停或恢复当前话题的推送，不影响别的话题使用同一个 RSS。
- `/import`、`/export`：在当前话题导入/导出对应订阅列表。

常规（General）话题使用与旧版相同的订阅范围（`thread_id=0`），普通群组、频道和私聊保持原有行为。

### 推文媒体与阅读排版

从 RSS/Atom 的 HTML、enclosure、Media RSS 中自动识别并发送静态图片、MP4 视频、GIF 动图、X/Twitter 的 `tweet_video` MP4 动图及音频。
单张图片、视频和动图以 Telegram 原生媒体消息发送；多张图片与视频可以组成原生相册（每组 2～10 个），超过十个会分批发送。
由于 Telegram 不允许动画与图片/视频混入同一 `sendMediaGroup`，动图按原文顺序单独发送。发送失败会回退为带原文链接的文字消息。

默认预览前 400 字；可以在 `config.yml` 中通过 `preview_text: 0` 禁用预览，或改成其他长度。
标题、原文链接和标签使用原有的 `message_tpl`、`message_mode` 配置，未配置自定义模板时使用更紧凑的消息排版。

**Telegraph 是可选功能。** 填写 `telegraph_token` 或 `telegraph_account` 后，文章正文可以生成 Telegraph 阅读链接；
当媒体说明超过 Telegram 字数限制、且 Telegraph 页面存在时，优先发送带阅读链接的简短说明。Telegraph 不用来代替 Telegram 原生视频发送，也不会替视频重新上传文件。
如果视频链接需要登录、已过期、超过 Telegram 限制或无法由 Telegram 服务器访问，仍需从原文打开。

**已有数据提示：**旧版 flowerss-bot 从未记录话题 ID，因此之前在任意话题创建的订阅无法自动判断原来的话题。它们在升级后会归属**常规（General）**话题。若需要把旧 RSS 分配到不同话题，在目标话题重新执行 `/sub <RSS URL>`；然后视需要在常规话题执行 `/unsub <RSS URL>`。相同 RSS 可以同时订阅在多个话题，不会重复抓取。

1. 下载配置文件

```bash
mkdir -p ~/flowerss
curl -fsSL https://raw.githubusercontent.com/FlanChanXwO/flowerss-bot/master/config.yml.sample \
  -o ~/flowerss/config.yml
```

2. 修改配置文件中的 `bot_token`。SQLite 使用默认 `./data.db` 即可，数据保存在挂载目录。

```bash
vim ~/flowerss/config.yml
```

3. 拉取已经构建好的镜像并启动：

```bash
docker pull ghcr.io/flanchanxwo/flowerss-bot:latest
docker run -d --name flowerss-bot --restart unless-stopped \
  -v "$HOME/flowerss:/root/.flowerss" \
  ghcr.io/flanchanxwo/flowerss-bot:latest
```

之后更新只需重新 `docker pull` 并重建容器。也可使用仓库中的 `docker-compose.yml`，
先将配置文件保存到 `./conf/config.yml`，执行 `docker compose pull && docker compose up -d --no-build`。

## 源码编译部署

```shell script
git clone https://github.com/indes/flowerss-bot && cd flowerss-bot
make build
./flowerss-bot
```



## 配置

根据以下模板，新建 `config.yml` 文件。

```yml
bot_token: XXX
#多个telegraph_token可采用数组格式：
# telegraph_token:
#  - token_1
#  - token_2
telegraph_token: xxxx
user_agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36
preview_text: 400
disable_web_page_preview: false
socks5: 127.0.0.1:1080
update_interval: 10
error_threshold: 100
telegram:
  endpoint: https://xxx.com/
mysql:
  host: 127.0.0.1
  port: 3306
  user: user
  password: pwd
  database: flowerss
sqlite:
  path: ./data.db
allowed_users:
  - 123
  - 234
```

配置说明：

| 配置项                     | 含义                                      | 是否必填                                       |
| --------------------------| ----------------------------------------- | ------------------------------------------ |
| bot_token                 | Telegram Bot Token                        | 必填                                       |
| telegraph_token           | Telegraph Token, 用于转存原文到 Telegraph   | 可忽略（不转存原文到 Telegraph ）          |
| preview_text              | 文字预览长度；有 Telegraph 时可附阅读链接   |可忽略（默认400, 0为禁用）                  |
| user_agent                | User Agent                                |可忽略                                     |
| disable_web_page_preview  | 是否禁用 web 页面预览                       | 可忽略（默认 false, true 为禁用）          |
| update_interval           | RSS 源扫描间隔（分钟）                      | 可忽略（默认 10）                          |
| error_threshold           | 源最大出错次数                              |可忽略（默认 100）                          |
| socks5                    | 用于无法正常 Telegram API 的环境            | 可忽略（能正常连接上 Telegram API 服务器） |
| mysql                     | MySQL 数据库配置                           | 可忽略（使用 SQLite ）                     |
| sqlite                    | SQLite 配置                               | 可忽略（已配置mysql时，该项失效）          |
| telegram.endpoint         | 自定义telegram bot api url                | 可忽略（使用默认api url）          |
| allowed_users             | 允许使用bot的用户telegram id，                        | 可忽略，为空时所有用户都能使用bot          |