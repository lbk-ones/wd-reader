# 新资源来源核验（2026-11-07）

按用户提供的 20 本小说核验新来源，不要求每站覆盖全部书目。表格只计入书名精确匹配、作者标签符合对照且有实际下载链接的结果；未命中不代表站点一定没有该书。

久久小说网的作者来自详情页；HF 目录仅提供文件名和大小，作者标签使用对照书单，不表示已核验文件全文。连载进度、版本和完整程度以实际文件为准。

| 小说 | 作者 | 久久小说网 | HF 目录 |
| --- | --- | --- | --- |
| 斗破苍穹 | 天蚕土豆 | 未精确命中 | 未精确命中 |
| 凡人修仙传 | 忘语 | 未精确命中 | [TXT](https://huggingface.co/datasets/NovelStory/NovelStory/blob/main/%E5%87%A1%E4%BA%BA%E4%BF%AE%E4%BB%99%E4%BC%A0.txt) |
| 诡秘之主 | 爱潜水的乌贼 | [TXT](https://www.ijjxsxzw.com/txt/41536.html) | 未精确命中 |
| 遮天 | 辰东 | 未精确命中 | 未精确命中 |
| 完美世界 | 辰东 | [TXT](https://www.ijjxsxzw.com/txt/21181.html) | 未精确命中 |
| 吞噬星空 | 我吃西红柿 | 未精确命中 | 未精确命中 |
| 仙逆 | 耳根 | 未精确命中 | 未精确命中 |
| 神墓 | 辰东 | 未精确命中 | 未精确命中 |
| 全职高手 | 蝴蝶蓝 | [TXT](https://www.ijjxsxzw.com/txt/15852.html) | 未精确命中 |
| 大奉打更人 | 卖报小郎君 | [TXT](https://www.ijjxsxzw.com/txt/38918.html) | [TXT](https://huggingface.co/datasets/NovelStory/NovelStory/blob/main/%E5%A4%A7%E5%A5%89%E6%89%93%E6%9B%B4%E4%BA%BA.txt) |
| 道诡异仙 | 狐尾的笔 | [TXT](https://www.ijjxsxzw.com/txt/51767.html) | [TXT](https://huggingface.co/datasets/NovelStory/NovelStory/blob/main/%E9%81%93%E8%AF%A1%E5%BC%82%E4%BB%99.txt) |
| 神秘复苏 | 佛前献花 | [TXT](https://www.ijjxsxzw.com/txt/44926.html) | 未精确命中 |
| 轮回乐园 | 那一只蚊子 | 未精确命中 | 未精确命中 |
| 圣墟 | 辰东 | [TXT](https://www.ijjxsxzw.com/txt/38482.html) | 未精确命中 |
| 全职法师 | 乱 | [TXT](https://www.ijjxsxzw.com/txt/49136.html) | 未精确命中 |
| 超神机械师 | 齐佩甲 | [TXT](https://www.ijjxsxzw.com/txt/36759.html) | 未精确命中 |
| 惊悚乐园 | 三天两觉 | [TXT](https://www.ijjxsxzw.com/txt/26403.html) | 未精确命中 |
| 我真没想重生啊 | 柳岸花又明 | [TXT](https://www.ijjxsxzw.com/txt/38047.html) | 未精确命中 |
| 仙工开物 | 蛊真人 | 未精确命中 | [TXT](https://huggingface.co/datasets/NovelStory/NovelStory/blob/main/%E4%BB%99%E5%B7%A5%E5%BC%80%E7%89%A9.txt) |
| 夜无疆 | 辰东 | 未精确命中 | 未精确命中 |

实际完整文件下载验证（测试目录，未加入用户书架）：

- 中文 TXT 目录 · HF：《仙工开物》，4,392,649 字节（约 4.2 MiB）。
- 久久小说网：《诡秘之主》，9,700,933 字节（约 9.3 MiB）。

本轮也检查了搜笔趣阁、无限小说网、星鱼小说、爱上阅读、我下小说网及久久小说网的其他域名等候选。部分只提供网盘跳转、搜索或下载超时，或要求验证，未作为默认下载来源加入。

可运行 `WD_READER_LIVE_NEW_SOURCES=1 go test ./internal/resourcecatalog -run TestLiveNewSourceBookList -v` 重新核验；网络检查默认关闭。
