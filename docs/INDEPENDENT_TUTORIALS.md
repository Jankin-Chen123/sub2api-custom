# 独立教程文档

教程网站已迁至 [aibaipiao-docs](https://github.com/Jankin-Chen123/aibaipiao-docs)，由独立的 Web 和 API 服务部署。

- 阅读入口：<https://aibaipiao.top/docs>
- 管理入口：<https://aibaipiao.top/docs/admin>，使用文档服务自己的管理登录。
- Notion HTML / Markdown 压缩包的导入、解析、预览、发布、历史版本和图片均由独立项目维护。
- 修改教程前端只需构建、发布独立项目，无需重建 Sub2API。

## 主站保留的集成

侧栏提供教程和文档管理的外部链接。旧 SPA 地址 `/docs`、`/docs/:section`、`/admin/documentation` 仅执行整页跳转，保留查询参数和锚点。原有 `/docs/batch-image` 是主站批量生图指南的历史别名，继续保留。

边缘反向代理负责将 `/docs` 及其子路径、`/api/v1/docs` 及其子路径、`/api/v1/docs-admin` 及其子路径交给独立服务。旧管理 API `/api/v1/admin/docs` 的兼容代理也由独立服务处理。完整配置与升级、回滚流程见独立仓库的 `docs/operations.md`。

Sub2API 后端不再注册教程读写接口，也不再初始化教程存储。主站的自定义菜单页面 `/api/v1/pages`、站点文档链接设置 `doc_url`、批量生图指南、法律与项目开发文档仍属于主项目。

## 旧数据

移除旧代码不会自动删除旧的 `data/documentation`。它仅作为迁移后的回滚材料保留；独立服务使用自己的数据目录。两个服务不能共享可写的文档目录。

清理旧数据前应完成迁移观察期并核验备份。若独立文档已发布新内容，回滚到旧版本前必须冻结写入并迁回、验证最新完整数据，不能直接恢复旧副本。
