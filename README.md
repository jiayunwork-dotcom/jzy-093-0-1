# rocalc — 小型苦咸水反渗透核算服务

一个常驻 HTTP 服务，固化反渗透（RO）过程核算内核：**van't Hoff 渗透压、净推动力、产水通量、回收率与全系统盐物料衡算**。提交一份具名工况档（或临时拼一份档）即可拿到自洽结果，不用再在 Excel 里来回倒腾。

- 语言：Go 1.22，HTTP 仅用标准库 `net/http`，零外部依赖
- 压力单位全程统一为 **bar**，浓度 **mol/L**，温度 **K**，流量 **L/h**；渗透压与工作压差之间不做任何隐式量纲换算
- 无页面，只提供 JSON 接口

## 内核关系

| 量 | 关系 |
|---|---|
| 渗透压 | π = i·C·R·T，R = 0.08314 L·bar/(mol·K)（NaCl 取 i=2） |
| 浓度解析 | 直接给摩尔浓度；或给质量浓度 g/L ÷ 摩尔质量 g/mol；两条路径同时给出时必须在容差内自洽 |
| 浓差极化 | 薄因子 β≥1：膜面跨膜渗透压差 Δπ = β·πf（忽略产水侧渗透压） |
| 净推动力 | NDP = Δp − Δπ |
| 产水流量 | Qp = A_mem·Lp·NDP |
| 回收率 | Y = Qp/Qf，必须落在开区间 (0,1) |
| 产水浓度 | r=1 时 Cp=0；r<1 时 Cp=(1−r)·Cf |
| 浓水浓度 | r=1：Cb = **Cf/(1−Y)**；r<1（守恒通式）：Cb = Cf·(1−(1−r)Y)/(1−Y) |
| 盐守恒 | Cf·Qf = Cp·Qp + Cb·Qb，Qb=Qf−Qp，每次核算都强制核对 |

**明确防住的坑**：浓水浓度绝不写成 Cf·(1−Y)（那种写法回收越高浓水越稀，盐守恒必崩），有专门测试守护。

### 拒绝条件（HTTP 422，带原因）

- 浓度为负、温度 T≤0、范特霍夫因子 i≤0、膜面积/渗透系数/进料流量非正
- 极化因子 β<1、盐截留率 r∉[0,1]
- 两种浓度表示换算后不自洽
- NDP<0（Δp 低于跨膜渗透压差）——**绝不返回正通量**
- NDP=0（如 Δp=πf 且无极化）——产水为零，回收率落不进 (0,1)
- Y≥1
- 盐物料守恒核对不过

### 交叉关系（均有测试）

- 只提温度 → π 升、同压差下 NDP 降、Qp 降
- 只提回收率 → 浓水浓度升
- r=1 → 产水含盐为零、盐全部进浓水
- Δp=πf 且 β=1 → Qp=0

## 包结构（按职责切开）

```
cmd/server/             服务启动入口，固定监听 :8080
internal/osmotic/       van't Hoff 渗透压（纯函数 + 合法性）
internal/casefile/      工况档模型、具名登记表（并发安全）、内置苦咸水档
internal/validate/      输入合法性、浓度量纲自洽、盐物料守恒核对
internal/membrane/      NDP / 通量 / 回收 / 浓水浓度 / Evaluate 编排
internal/api/           net/http JSON 接口
```

`membrane.Evaluate` 是纯函数：同一份档任何时候结果一致；不同档之间没有共享状态，同一进程可并存任意多份互不串名的档。

## HTTP 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET  | `/healthz` | 健康检查 |
| GET  | `/cases` | 列出已登记工况档 |
| GET  | `/cases/{name}` | 查看某份档 |
| POST | `/cases` | 登记一份具名工况档（重名 409） |
| POST | `/calculate` | 临时提交一份档直接核算，不落登记 |
| POST | `/cases/{name}/calculate` | 点名登记表中的档核算 |

状态码：200 成功 / 201 已登记 / 400 JSON 或名称问题 / 404 档不存在 / 409 重名 / 422 物理上不成立（消息带具体原因）。

### 示例

```bash
# 点名内置苦咸水档（约 5.84 g/L NaCl，25℃，π≈4.96 bar）
curl -s -X POST localhost:8080/cases/brackish-demo/calculate

# 临时拼一份档（质量浓度路径也可，只要量纲自洽）
curl -s -X POST localhost:8080/calculate -H 'Content-Type: application/json' -d '{
  "name": "adhoc",
  "feed": {"concentration": {"solute":"NaCl","molarity_mol_L":0.1,"van_t_hoff_factor":2},
           "temperature_K": 298.15, "feed_flow_L_p_h": 1000},
  "membrane": {"area_m2": 2.5, "hydraulic_permeability_L_p_h_p_m2_p_bar": 12},
  "operating": {"applied_pressure_bar": 12, "polarization_factor": 1.1, "salt_rejection": 0.99}
}'
```

返回中关键字段：`feed.feed_osmotic_pressure_bar`、`operating.net_driving_pressure_bar`、
`membrane.product_flow_L_p_h` / `recovery`、`salt_balance.*`（三股盐量与守恒残差 `residual_mol_p_h`）。

## 本地开发

```bash
go test -race -count=1 ./...   # 全套自动化测试
go vet ./...
go run ./cmd/server            # 启动服务，:8080
```

## Docker

镜像构建过程中会真实执行 `go vet` 和 `go test -race`，测试不过则镜像构建失败；运行镜像为 distroless、非 root，固定暴露 8080。

```bash
docker build -t rocalc .
docker run --rm -p 8080:8080 rocalc
```

## 内置档手算对照

brackish-demo（Cf=0.10 mol/L，i=2，T=298.15 K，β=1.10，Δp=12 bar，A=2.5 m²，Lp=12，r=0.99）：

```
πf = 2×0.10×0.08314×298.15 = 4.95764 bar
Δπ = 1.10×4.95764          = 5.45340 bar
NDP= 12 − 5.45340          = 6.54660 bar
Qp = 2.5×12×6.54660        = 196.398 L/h，Y = 0.196398
Cp = 0.001 mol/L，Cb       = 0.1241953 mol/L
盐守恒残差 ≈ 1e-14 mol/h（浮点噪声）
```
