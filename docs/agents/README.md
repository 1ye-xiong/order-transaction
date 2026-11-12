# 题目01 Agent与Skill使用方式

本项目以根AGENTS.md为共同约束，三个角色文件是独立会话启动指令。不是注册了三个正在运行的Agent，也不代替真实独立会话。实际任务不要求同时执行，不应共享一个会话扮演三种角色签发合格结论。

## 原始来源
题面：`C:/Users/35795/Desktop/Go项目业务场景题.md`，读取全部通用要求和题目01。
通用要求：`C:/Users/35795/AppData/Roaming/Tencent/Marvis/User/oAN1i2eqcOf5M-OU65nk2qrtP-l4/workspace/conv_a6efcc9bc8bf4a49a843f80be8e71068/output/Go业务场景题-10条通用要求大白话版.docx`。
这些绝对路径仅适用于当前机器；迁移时由用户提供原始来源或经确认的副本，不把本项目摘要当作原题替代品。

## 启动三个角色
开发新会话：打开本项目，读取AGENTS.md和docs/agents/developer.md，指定真实工作项、当前任务和版本，按其交接要求执行。
测试另开会话：读取AGENTS.md和docs/agents/tester.md，提供原题与明确代码快照，独立执行测试，不只读取开发总结。
验收再次另开会话：读取AGENTS.md和docs/agents/acceptor.md，提供最终代码和全部测试及发布证据，阶段结论不能当整题结论。

## 两个Skill
标准项目目录为`.agents/skills/<name>/SKILL.md`。工具支持项目级Skill发现时重新打开项目或会话，再选择对应Skill；不支持时显式要求读取该SKILL.md并按步骤执行。工具未发现时不能声称自动安装成功。本次没有改全局配置。
业务检查读取trade-business-check，验收读取topic01-acceptance-check。实际输入、执行、结果、修改复查和局限写根skill-usage.md。新会话须记录实际加载，不复制本次记录冒充调用。

## 必留记录
每次记录时间、角色、会话标识（不可获取如实填）、任务及真实工作项、AGENTS和Skill版本及摘要、提交和工作区差异、输入来源、实际命令原始输出、结论、学生审核、修正及复验位置。可见会话Markdown和JSON或JSONL事件按时间保存，脱敏，不采集隐藏推理。
本次改动及检查见docs/agent-skill-changes.md；未运行独立测试验收，未修改业务、提交、合并或部署。
