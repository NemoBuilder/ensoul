# Credits & Attribution

本目录下的方法论资料**完全来源于**开源项目 **x-mentor-skill**：

- 仓库：https://github.com/alchaincyf/x-mentor-skill
- 作者：Huashu (花叔) [@AlchainHust](https://x.com/AlchainHust)
- 版本：v2.0（2026-04-06 调研版）
- 协议：MIT License（见 `LICENSE`）

## 内容溯源

原作者基于以下来源蒸馏：

1. **6位顶级创作者方法论**：Nicolas Cole、Dickie Bush、Sahil Bloom、Justin Welsh、Dan Koe、Alex Hormozi
2. **X 平台开源算法源码分析**（2026 年 4 月）
3. **AI / 科技赛道专精策略**

提炼出：
- 6 个核心心智模型
- 10 条决策启发式
- 5 份操作手册（writing / algorithm / growth / quality / mental-models）
- 1 份调研索引（references/research/，本仓库未拉取）

## Ensoul 中的使用方式

- 我们将 5 份 reference + 6 mental model + 10 heuristic + 1 routing 切片，写入数据库表 `mentor_methodologies`，作为 Vibe Write 导师层的**默认方法论包**。
- 所有 record 的 `source` 字段统一标记为 `x-mentor-skill@v2.0`，`source_url` 指向原仓库。
- 我们**不修改**这些原始内容；任何 Ensoul 自有补充将以 `source=internal-ensoul` 单独入库，避免污染。
- 升级时保留 `version` 字段，新版以 `x-mentor-skill@v3.0` 等方式新增，旧版 `enabled=false` 但保留以追溯。

## 致谢

感谢花叔将这套高密度方法论以 MIT 协议开源，让 Ensoul 能够以一个真正"懂行"的导师起步，而不是从零摸索。

— Ensoul Team, 2026-04
