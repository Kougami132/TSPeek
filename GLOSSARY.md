# TSPeek

TeamSpeak 服务器实时监控仪表板。通过 ServerQuery 协议获取服务器状态、频道树与客户端信息，并向 Web UI 提供实时展示与变动记录。

## Language

**Snapshot**:
某一时刻 TeamSpeak 虚拟服务器完整状态的数据切片，包含服务器基础信息、频道列表、在线客户端及权限组。
_Avoid_: Dump, state

**Activity Log**:
以时间序列记录客户端进入服务器、离开服务器及在频道间移动的流转事件。
_Avoid_: Audit log, server log, history

**Activity Event**:
客户端在特定时间点发生的状态跃迁原子事实，包含加入、离开、移动和改名四种动作。
_Avoid_: Action, operation, notification

**Client Identity**:
以 TeamSpeak 唯一身份标识（UID）为核心的用户凭证，用于在昵称修改和同名场景下唯一识别特定客户端。
_Avoid_: DatabaseID, Nickname
