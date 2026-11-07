# 资源下载

书架顶部「资源下载」提供「输入书名 → 选择文件格式 → 下载 → 自动加入书架」的流程。

默认接入「久久小说网」「中文 TXT 目录 · HF」「速读谷」「爱下电子书」、Internet Archive 和 Project Gutenberg：先查询书名或目录文件名，再读取实际文件列表，展示 TXT、EPUB、MOBI、AZW3 或包含这些文件的 ZIP。爱下电子书的 TXT ZIP 下载后自动解压并加入书架；速读谷提供分卷 TXT，文件名显示章节范围，每个分卷分别下载与入库，连载作品以来源当前提供的章节为准。Internet Archive 的受限资源、私有文件、标记为加密/LCP 的文件及 Gutenberg 的购买链接不会作为下载选项。搜索最多展示每个来源的 8 项资源，同一资源的不同文件或版本分别列出，文件大小未知时不显示大小。

中文来源优先显示同名作品，支持配置书名别名与作者提示。例如「我有一座冒险屋／我有一座恐怖屋」会合并检索；「神秘复苏」额外使用作者「佛前献花」查找原作，避免只有同人作品。对带分页的搜索页，可在指定页数内读取站点提供的分页链接，不只检查第一页。

点击「下载」后可在「下载任务」查看进度、取消或失败重试。关闭面板仍会继续下载，成功后调用现有本地导入流程，保留原文件并加入书架；同名文件沿用现有导入机制保留两份。加入书架阶段不支持取消。任务仅在本次应用运行中保留，不支持应用重启后的断点续传。

## 独立配置

配置位于 `books/resource-sources.json`，与应用的 books 目录一致（Windows 工作目录、macOS/Linux 用户主目录）。首次使用从内置默认配置创建，保留已有配置。可在「资源配置」中查看绝对路径、编辑并保存，或外部编辑后重新加载。

```json
{
  "version": 1,
  "sources": [
    {
      "id": "gutenberg",
      "name": "Project Gutenberg",
      "enabled": true,
      "kind": "gutenberg",
      "description": "经典文学电子书",
      "searchUrl": "https://www.gutenberg.org/ebooks/search/?query={query}&format=opds",
      "detailUrl": "https://www.gutenberg.org/ebooks/{id}.opds",
      "bookUrl": "https://www.gutenberg.org/ebooks/{id}"
    }
  ]
}
```

- `kind` 支持 `gutenberg`（OPDS）、`archive`（Internet Archive API）、`html`（可配置的页面链接解析）和 `file-index`（公开 JSON 文件目录）。
- `searchUrl` 使用 `{query}`；`detailUrl`、`bookUrl` 使用 `{id}`。
- `html` 的 `searchMethod` 默认为 GET；设为 `POST` 时使用 `searchForm` 中的表单字段，`{query}` 在 URL 或表单值中均可使用，按标准 URL 表单编码发送，适用于久久小说网等站点。
- `file-index` 使用 `indexUrl` 读取数组形式的文件目录，每项包含 `path`、`size`、`type`（文件为 `file`）。`downloadUrl`、`bookUrl` 包含 `{file}`，目录路径逐段 URL 编码。仅筛选可读取格式、大小在限制内的文件，排除父路径和可执行文件。目录缓存 5 分钟，最多缓存 30 个地址；HF 配置读取前 1000 项文件目录。
- 文件目录按文件名匹配书名，`authorHints` 仅提供配置中的作者标签，不能代替对文件内容、版本或完整程度的核验。
- `archive` 另需 `downloadUrl`，包含 `{id}` 和 `{file}`。书名会转换为带引号的检索词；标识符和文件名分别进行 URL 编码。
- `html` 通过 `bookLinkPattern` 匹配搜索页中的书籍 URL 路径，通过 `downloadLinkPattern` 匹配详情页中的完整下载 URL；均为 Go 正则表达式。仅跟随同站书籍详情链接。
- `html` 的 `titleMeta`、`authorMeta` 对应页面的 meta property/name。`zipFormat: "txt"` 表示下载链接是包含 TXT 的 ZIP，不执行包内文件，只提取指定格式中最大的电子书，忽略可执行文件和符号链接。
- 没有 meta 信息时可用 `titlePattern`、`authorPattern` 从 HTML 的第一个捕获组提取标题、作者；不设置 meta 键时不会让其他 meta 标签覆盖提取结果。
- `downloadPagePattern` 可匹配同站下载页的 URL 路径，在详情页没有直接文件链接时再跟随一层下载页，不执行页面脚本或跨站跳转。
- 对没有扩展名的下载接口，可指定 `downloadFormat: "txt"`；`fileNameFromLink: true` 优先保留链接文字中的文件名，适用于「1–500 章」「501–1000 章」等分卷链接。
- `html` 的 `aliases` 为书名到别名数组的映射，最多展开 3 个书名；`authorHints` 为书名到作者的映射，用于附加检索与排序，不会隐去其他同名作品。
- `maxSearchPages` 控制每个检索词最多访问多少页（默认 1，最大 10，中文来源默认 4），仅跟随页面中查询条件一致的 `page` 分页链接。旧作可能被近期同人挤到后页，因此优先检查站点给出的末页。
- 每来源查询超时 20 秒，目录响应限制 4 MiB；每项最多 12 个下载文件。
- 同时下载 2 个文件，最多排队 20 项，每个任务超时 5 分钟，单文件限制 100 MiB。
- 下载会拒绝网页、验证码页、空文件及明显格式不符的 EPUB/MOBI 文件，失败文件不进入书架。
- ZIP 压缩大小和解压大小均限制 100 MiB，最多 200 个条目；输出名称由所选书名生成，不采用包内路径。取消、校验失败或格式不符时不加入书架。

可以用「红楼梦」体验中文资源，用「Alice」体验 Gutenberg。文件清单能确认资源提供了下载链接，但链接仍可能因站点变化、网络问题或权限变化而下载失败；界面会保留具体错误供重试。

## 中文站核验（2026-11-07）

速读谷已通过书名与作者核验：「时停起手，邪神也得给我跪下！」（六个葫芦，3 个分卷）、「斗破苍穹」（天蚕土豆，4 个分卷）、「斗罗大陆」（唐家三少，2 个分卷）、「仙逆」（耳根，5 个分卷）。已实际下载「时停起手」第一个分卷 TXT 到测试目录。

「神印王座」原作（唐家三少）由爱下电子书检索和下载列表核验通过；速读谷只返回「神印王座II皓月当空」，因此不能视为原作命中。界面把非同名结果标为「相关作品」，同名结果优先显示。搜索与文件列表读取通过实际接口验证，没有向用户书架添加测试文件。

其他候选站包括书海阁、TXT8848、八零电子书、精品小说和对小说网。本次检查中部分搜索接口返回错误、超时或验证页面，因此未作为默认下载来源启用；页面能打开或搜索引擎能检索到，不代表可以完成应用内下载。

新增来源已用用户提供的 20 本书进行核验：久久小说网精确命中 11 本，HF 目录命中 4 本，合计覆盖 13 本，并分别实际下载《诡秘之主》和《仙工开物》作为验证。逐本结果见 [来源核验记录](resource-source-check-2026-11-07.md)。

接口参考：[Internet Archive Metadata API](https://archive.org/developers/md-read.html)、[Project Gutenberg OPDS](https://www.gutenberg.org/ebooks/search/?format=opds)。
