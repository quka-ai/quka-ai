# 基于对话式RAG的新型电商模式设计方案

## 一、核心理念

### 1.1 传统电商 vs 对话式电商

**传统电商模式:**
```
用户 → 浏览商品列表 → 筛选条件 → 点击商品 → 查看详情 → 加入购物车 → 结算 → 支付
      (冷冰冰)      (繁琐)     (信息过载)   (决策困难)
```

**对话式电商模式(基于RAG):**
```
用户 → 自然语言描述需求 → AI理解意图 → RAG检索知识库 → AI推荐并解释 → 用户确认 → AI调用Skill下单 → 完成
      (像朋友聊天)       (智能理解)   (精准匹配)      (有温度的推荐)    (无缝衔接)
```

### 1.2 核心优势

**对于宠物零食销售场景:**

1. **情感化交互**
   - 用户: "我家泰迪3个月大,最近有点拉肚子,有什么零食推荐吗?"
   - AI: "主人别担心哦~ 幼犬肠胃比较敏感,我给您推荐几款易消化的零食..."
   - 传统电商: 用户需要自己搜索、筛选、对比,不知道选哪个

2. **专业知识赋能**
   - AI从知识库中检索: 营养成分、适用范围、客户评价、喂养建议
   - 相当于每个用户都有专属的宠物营养师
   - 传统电商: 用户只能看冰冷的商品描述

3. **无缝购物体验**
   - 聊天过程中直接下单,无需跳转
   - AI: "这款鸡肉粒很适合您家毛孩子,我帮您下单2包100g装的吧?"
   - 用户: "好的"
   - AI调用Skill完成下单

4. **持续的关系维护**
   - 购买后AI主动询问: "主人,上次买的鸡肉粒您家泰迪吃得怎么样?"
   - 根据消费记录推荐: "您上次买的快吃完了吧,要不要再来一单?"

---

## 二、系统架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                    QukaAI RAG 系统(核心不变)                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │ 知识库    │  │ 聊天系统  │  │ 向量检索  │                  │
│  └──────────┘  └──────────┘  └──────────┘                  │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│              知识库内容扩展(商品作为知识条目)                    │
│  ┌──────────────────────────────────────────────┐          │
│  │ 商品知识库                                     │          │
│  │ - 商品名称、描述、成分、营养、适用范围          │          │
│  │ - 价格、库存、规格信息                        │          │
│  │ - 客户评价、常见问题                          │          │
│  │ (以Markdown格式存储,支持RAG检索)              │          │
│  └──────────────────────────────────────────────┘          │
│  ┌──────────────────────────────────────────────┐          │
│  │ 喂养知识库                                     │          │
│  │ - 不同犬种的营养需求                          │          │
│  │ - 年龄阶段的喂养建议                          │          │
│  │ - 常见健康问题与食疗                          │          │
│  └──────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                  AI Agent 能力增强                            │
│  ┌──────────────────────────────────────────────┐          │
│  │ Tool Calling (现有能力)                        │          │
│  │ - search_knowledge (检索商品/知识)             │          │
│  │ - query_order (查询订单状态)                  │          │
│  │ - track_logistics (物流跟踪)                  │          │
│  └──────────────────────────────────────────────┘          │
│  ┌──────────────────────────────────────────────┐          │
│  │ Skill 系统(新增电商Skill)                      │          │
│  │ - /place_order (下单)                         │          │
│  │ - /add_to_cart (加入购物车)                   │          │
│  │ - /cancel_order (取消订单)                    │          │
│  │ - /request_refund (申请退款)                  │          │
│  └──────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│              轻量级订单系统(后端支撑)                          │
│  - 订单数据表(最小必要字段)                                   │
│  - 简单的库存管理                                            │
│  - 支付状态跟踪                                              │
│  - 物流信息关联                                              │
│  (用户无感知,全部通过AI交互)                                  │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 关键设计原则

1. **QukaAI保持纯正的RAG系统**
   - 不添加传统电商UI组件
   - 商品信息作为Knowledge存储
   - 所有交互通过聊天完成

2. **订单系统作为"后端支撑"**
   - 用户不直接访问订单管理界面
   - 通过AI对话查询订单: "我的订单到哪了?"
   - 通过AI对话下单: "帮我下单2包鸡肉干"

3. **AI是唯一的交互界面**
   - 售前咨询: AI检索知识库推荐
   - 下单: AI调用Skill
   - 售后: AI查询订单并处理

---

## 三、核心实现方案

### 3.1 商品作为Knowledge存储

**不新增Product表,而是利用现有Knowledge表:**

#### 商品知识条目示例

```markdown
# 手工鸡肉粒 (幼犬配方)

## 商品编码
SKU: PET-CHICKEN-100G-001

## 商品简介
精选新鲜鸡胸肉手工制作,低温烘干锁住营养,专为3-12月龄幼犬设计。肉质软嫩易咀嚼,帮助幼犬健康成长。

## 主要成分
- 鸡胸肉 75%
- 鸡肝 10%
- 南瓜 8%
- 胡萝卜 5%
- 其他 2% (天然维生素)

## 营养成分(每100g)
- 蛋白质: 28g
- 脂肪: 8g
- 水分: 10g
- 粗纤维: 3g
- 钙: 1.2g

## 适用范围
- **年龄**: 3-12月龄幼犬
- **犬种**: 全犬种
- **体型**: 小型犬、中型犬、大型犬
- **健康状况**: 肠胃敏感、发育期、挑食

## 产品特点
- ✓ 100%手工制作
- ✓ 无添加剂、无防腐剂
- ✓ 低温烘干,营养不流失
- ✓ 软硬适中,幼犬易咀嚼
- ✓ 补充蛋白质和钙质

## 喂养建议
- **小型犬**(5kg以下): 每天10-15g,分2-3次
- **中型犬**(5-15kg): 每天15-25g,分2-3次
- **大型犬**(15kg以上): 每天25-40g,分2-3次

**注意**: 作为零食,不应超过每日总摄入量的10%

## 规格与价格
- 100g装: ¥28.00 (会员价 ¥25.00)
- 500g装: ¥128.00 (会员价 ¥115.00)
- 1000g装: ¥238.00 (会员价 ¥215.00)

## 库存状态
- 100g装: 库存充足 (50+)
- 500g装: 库存紧张 (仅剩12包)
- 1000g装: 库存充足 (30+)

## 保质期与保存
- 保质期: 45天(开封后请在7天内食用完毕)
- 保存方法: 密封,阴凉干燥处保存

## 客户评价精选
> "我家泰迪特别挑食,这个鸡肉粒一给就吃光了!" - 李女士 ⭐⭐⭐⭐⭐

> "软硬刚好,小狗咬着不费劲,而且真的是纯鸡肉,放心!" - 张先生 ⭐⭐⭐⭐⭐

## 常见问题
**Q: 幼犬多大可以开始吃?**
A: 建议3个月以上开始尝试,初次喂食先给少量观察适应情况。

**Q: 可以当主食吗?**
A: 不建议,这是零食,应该配合主粮一起喂养。

**Q: 过敏犬可以吃吗?**
A: 如果对鸡肉过敏不建议,可以尝试我们的牛肉系列。

---
**元数据**:
- 类别: 鸡肉类 / 零食 / 幼犬专用
- 标签: #幼犬 #易消化 #补钙 #无添加 #手工制作
- 推荐指数: ⭐⭐⭐⭐⭐
- 复购率: 85%
```

**知识库字段映射:**

```go
knowledge := &types.Knowledge{
    SpaceID:     spaceID,
    Kind:        types.KNOWLEDGE_KIND_TEXT,
    Resource:    "product",  // 标记为商品类型
    Title:       "手工鸡肉粒(幼犬配方)",
    Tags:        []string{"商品", "鸡肉类", "幼犬", "零食"},
    Content:     markdownContent,  // 上面的Markdown内容
    ContentType: types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
    Source:      "ecommerce",

    // 扩展字段存储结构化数据
    Summary:     "精选鸡胸肉手工制作的幼犬零食,低温烘干营养丰富",

    // 可以在Meta中存储SKU、价格等结构化信息便于查询
    // (通过KnowledgeMeta表)
}
```

### 3.2 Skill系统实现电商能力

#### Skill 1: place_order (下单)

**用户体验:**
```
用户: "那就来2包100g的鸡肉粒吧"
AI: "好的主人,我帮您下单:
     - 手工鸡肉粒(100g装) x 2
     - 单价: ¥25.00 (会员价)
     - 运费: ¥0 (满50包邮)
     - 总计: ¥50.00

     请确认您的收货信息:
     张三 138****8000
     北京市朝阳区xxx小区xx号楼xxx室

     确认无误请回复'确认下单',需要修改请告诉我~"

用户: "确认下单"
AI: [调用 /place_order skill]
    "订单已生成!订单号: PS260110A1B2C3
     请扫码支付 ¥50.00 [显示收款码]
     支付完成后请将截图发给我,我会尽快为您确认订单~"
```

**Skill实现:**

```go
// pkg/skills/ecommerce/place_order.go

type PlaceOrderSkill struct {
    core *core.Core
}

func (s *PlaceOrderSkill) Name() string {
    return "place_order"
}

func (s *PlaceOrderSkill) Description() string {
    return "为用户创建订单,完成下单流程"
}

func (s *PlaceOrderSkill) Parameters() []types.SkillParameter {
    return []types.SkillParameter{
        {
            Name:        "items",
            Type:        "array",
            Description: "订单商品列表",
            Required:    true,
            Schema: map[string]interface{}{
                "items": {
                    "sku":      "string",  // SKU编码
                    "quantity": "integer", // 数量
                },
            },
        },
        {
            Name:        "address",
            Type:        "object",
            Description: "收货地址信息",
            Required:    true,
        },
        {
            Name:        "note",
            Type:        "string",
            Description: "客户备注",
            Required:    false,
        },
    }
}

func (s *PlaceOrderSkill) Execute(ctx context.Context, args map[string]interface{}) (*types.SkillResult, error) {
    // 1. 解析参数
    items := args["items"].([]OrderItem)
    address := args["address"].(Address)

    // 2. 检查库存
    for _, item := range items {
        available, err := s.checkInventory(ctx, item.SKU, item.Quantity)
        if err != nil || !available {
            return &types.SkillResult{
                Success: false,
                Message: fmt.Sprintf("抱歉主人,%s库存不足,当前仅剩%d份", item.Name, currentStock),
            }, nil
        }
    }

    // 3. 计算价格
    totalAmount := s.calculateTotalAmount(ctx, items, userID)

    // 4. 创建订单
    order := &Order{
        OrderNo:     GenerateOrderNo(),
        UserID:      userID,
        Items:       items,
        TotalAmount: totalAmount,
        Address:     address,
        Status:      "pending",
    }

    if err := s.core.Store().OrderStore().Create(ctx, order); err != nil {
        return &types.SkillResult{
            Success: false,
            Message: "下单失败,请稍后重试",
        }, err
    }

    // 5. 锁定库存
    s.lockInventory(ctx, items)

    // 6. 返回支付信息
    return &types.SkillResult{
        Success: true,
        Message: fmt.Sprintf("订单创建成功!订单号: %s", order.OrderNo),
        Data: map[string]interface{}{
            "order_no":      order.OrderNo,
            "total_amount":  totalAmount,
            "payment_qrcode": s.getPaymentQRCode(),
        },
    }, nil
}
```

#### Skill 2: query_order (查询订单)

```go
// pkg/skills/ecommerce/query_order.go

func (s *QueryOrderSkill) Execute(ctx context.Context, args map[string]interface{}) (*types.SkillResult, error) {
    // 可以通过订单号或用户ID查询
    orderNo := args["order_no"].(string)

    order, err := s.core.Store().OrderStore().GetByOrderNo(ctx, orderNo)
    if err != nil {
        return &types.SkillResult{
            Success: false,
            Message: "未找到该订单",
        }, nil
    }

    // 查询物流信息
    logistics, _ := s.getLogisticsInfo(ctx, order.TrackingNo)

    return &types.SkillResult{
        Success: true,
        Data: map[string]interface{}{
            "order":     order,
            "logistics": logistics,
        },
    }, nil
}
```

**AI调用示例:**
```
用户: "我的订单PS260110A1B2C3到哪了?"
AI: [调用 query_order skill]
    "主人,您的订单目前状态:
     - 订单状态: 运输中
     - 物流公司: 顺丰速运
     - 快递单号: SF1234567890
     - 最新动态: 快件已到达北京转运中心 (1小时前)

     预计明天就能送到啦,请保持手机畅通哦~"
```

#### Skill 3: add_to_cart (加入购物车)

```go
func (s *AddToCartSkill) Execute(ctx context.Context, args map[string]interface{}) (*types.SkillResult, error) {
    // 添加商品到购物车
    // 用户可以先"收藏"几个,之后一起下单
}
```

### 3.3 Tool Calling 增强

#### ProductRecommendationTool (智能推荐)

```go
// pkg/ai/tools/ecommerce/recommendation.go

type ProductRecommendationTool struct {
    core    *core.Core
    spaceID string
}

func (t *ProductRecommendationTool) Info(ctx context.Context) (*tool.ToolInfo, error) {
    return &tool.ToolInfo{
        Name: "recommend_products",
        Desc: "根据用户需求(犬种、年龄、健康状况等)智能推荐商品",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "pet_info": {
                Type:     schema.Object,
                Desc:     "宠物信息",
                Required: false,
            },
            "requirements": {
                Type:     schema.String,
                Desc:     "用户需求描述",
                Required: true,
            },
        }),
    }, nil
}

func (t *ProductRecommendationTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
    // 1. 解析参数
    var params struct {
        PetInfo      PetInfo `json:"pet_info"`
        Requirements string  `json:"requirements"`
    }
    json.Unmarshal([]byte(argumentsInJSON), &params)

    // 2. 构建RAG查询
    query := fmt.Sprintf(`
        用户需求: %s
        宠物信息: 品种=%s, 年龄=%s, 体型=%s, 健康状况=%s
        请推荐最合适的商品
    `, params.Requirements, params.PetInfo.Breed, params.PetInfo.Age, params.PetInfo.Size, params.PetInfo.HealthConcern)

    // 3. 向量检索知识库(筛选resource=product)
    results, err := t.core.KnowledgeLogic().VectorSearch(ctx, t.spaceID, query, &types.SearchOptions{
        Filters: map[string]interface{}{
            "resource": "product",  // 只检索商品类知识
        },
        TopK: 5,
    })

    // 4. 提取商品信息
    var products []ProductInfo
    for _, result := range results {
        product := t.extractProductInfo(result.Content)  // 从Markdown解析
        products = append(products, product)
    }

    // 5. 返回推荐结果(JSON格式)
    return json.Marshal(map[string]interface{}{
        "recommendations": products,
        "reason":          "基于您家毛孩子的情况,这些商品最适合...",
    })
}
```

**AI使用示例:**
```
用户: "我家金毛5个月大,最近运动量大,吃什么零食好?"

AI思考过程:
1. [调用 recommend_products tool]
   参数: {
     "pet_info": {"breed": "金毛", "age": "5个月", "size": "大型犬"},
     "requirements": "运动量大,需要补充能量"
   }

2. Tool返回:
   {
     "recommendations": [
       {
         "name": "手工牛肉粒(幼犬配方)",
         "sku": "PET-BEEF-100G-001",
         "price": 32.00,
         "reason": "高蛋白,适合运动量大的幼犬",
         "stock": "充足"
       },
       {
         "name": "鸡肉+牛肉混合粒",
         "sku": "PET-MIX-100G-001",
         "price": 35.00,
         "reason": "营养均衡,补充多种氨基酸",
         "stock": "库存紧张"
       }
     ]
   }

3. AI回复:
   "主人,您家金毛正处于快速发育期,运动量大需要补充充足的蛋白质!
    我为您推荐两款零食:

    ⭐ 推荐一: 手工牛肉粒(幼犬配方)
    - 价格: ¥32/100g (会员价 ¥28)
    - 特点: 纯牛肉制作,蛋白质含量32%,专为大型犬设计
    - 适合原因: 金毛运动量大,牛肉的高蛋白能帮助肌肉发育
    - 库存: 充足

    ⭐ 推荐二: 鸡肉+牛肉混合粒
    - 价格: ¥35/100g
    - 特点: 鸡肉和牛肉双重蛋白,营养更均衡
    - 适合原因: 多种肉类提供不同氨基酸,促进全面发育
    - 库存: ⚠️ 仅剩8包

    建议您先尝试牛肉粒,大多数金毛都很喜欢!要不要来一包试试?"
```

---

## 四、最小数据库设计

虽然用户不直接访问,但后端仍需要最小化的订单表支撑:

### 4.1 Order表(简化版)

```sql
CREATE TABLE ecommerce_order (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_no VARCHAR(32) UNIQUE NOT NULL,

    space_id UUID NOT NULL,
    user_id UUID NOT NULL,

    -- 订单快照(JSON格式,包含商品、价格、收货信息等)
    order_snapshot JSONB NOT NULL,

    -- 订单状态
    status VARCHAR(20) DEFAULT 'pending',

    -- 支付信息
    payment_method VARCHAR(50),
    payment_time BIGINT,
    transaction_id VARCHAR(100),

    -- 物流信息
    logistics JSONB,

    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,

    FOREIGN KEY (space_id) REFERENCES space(space_id),
    FOREIGN KEY (user_id) REFERENCES "user"(id)
);

CREATE INDEX idx_ecommerce_order_user_id ON ecommerce_order(user_id);
CREATE INDEX idx_ecommerce_order_no ON ecommerce_order(order_no);
```

**设计理念:**
- 所有详细信息放在JSONB字段,灵活且无需频繁修改表结构
- AI查询订单时,直接解析JSON返回给用户
- 保持最小化,避免过度设计

### 4.2 Inventory表(简化版)

```sql
CREATE TABLE ecommerce_inventory (
    sku VARCHAR(100) PRIMARY KEY,
    space_id UUID NOT NULL,

    knowledge_id UUID,  -- 关联商品知识库

    stock INTEGER NOT NULL DEFAULT 0,
    available_stock INTEGER NOT NULL DEFAULT 0,
    locked_stock INTEGER NOT NULL DEFAULT 0,

    updated_at BIGINT NOT NULL,

    FOREIGN KEY (knowledge_id) REFERENCES knowledge(id)
);
```

---

## 五、对话式购物流程示例

### 5.1 完整购物对话

```
=== 售前咨询阶段 ===

用户: "你好"
AI: "主人您好呀!我是您的专属宠物营养顾问,有什么可以帮您的吗?😊"

用户: "我家泰迪3个月,有点挑食,推荐点零食"
AI: [调用 recommend_products tool]
    "3个月的泰迪宝宝正是长身体的时候呢!挑食的话要选择适口性好的零食来引导~

     我给您推荐这款:

     🦴 手工鸡肉粒(幼犬配方)
     ✓ 纯鸡胸肉手工制作,肉香浓郁
     ✓ 软硬适中,幼犬容易咀嚼
     ✓ 低温烘干,保留天然肉香
     ✓ 无添加剂,妈妈们都很放心

     [显示商品图片]

     💰 价格: 100g装 ¥28 (会员价¥25)
            500g装 ¥128 (会员价¥115,更划算)

     📦 库存: 充足
     ⭐ 好评率: 98% (很多泰迪妈妈都回购了)

     这款95%的泰迪宝宝都爱吃,要不要试试看?"

=== 了解详情阶段 ===

用户: "成分都有什么?"
AI: [从知识库检索商品详情]
    "这款鸡肉粒的成分非常纯粹:

     主要成分:
     • 鸡胸肉 75% (人类食用级别)
     • 鸡肝 10% (补充维生素A)
     • 南瓜 8% (膳食纤维,助消化)
     • 胡萝卜 5% (补充胡萝卜素)
     • 天然维生素 2%

     营养成分(每100g):
     • 蛋白质 28g
     • 脂肪 8g (低脂配方)
     • 钙 1.2g (促进骨骼发育)

     ⚠️ 不含防腐剂、不含人工色素、不含香精

     您看这个成分放心吗?"

用户: "一天吃多少合适?"
AI: [从知识库检索喂养建议]
    "3个月大的泰迪属于小型犬幼犬,建议:

     每天喂食量: 10-15g (大约10-15粒)
     喂食次数: 分2-3次喂
     最佳时间: 饭后1小时或训练奖励时

     💡 温馨提示:
     - 零食摄入量不要超过每日总食量的10%
     - 初次喂食先给2-3粒,观察适应情况
     - 如果宝宝吃得很欢,可以用来做训练奖励哦

     100g装大约能吃1周左右,500g装更划算能吃一个多月~"

=== 下单阶段 ===

用户: "那先来一包100g的试试"
AI: "好嘞!我帮您加入购物车:

     🛒 购物车:
     • 手工鸡肉粒(幼犬配方) 100g x1
       单价: ¥25.00 (会员价已优惠¥3)

     运费: ¥0 (满50包邮,差¥25就包邮啦)
     总计: ¥25.00

     💡 小建议: 再加一包就包邮了,而且500g装更划算哦,要不要考虑升级?"

用户: "那就500g吧"
AI: "明智的选择!500g装性价比超高~

     🛒 已更新购物车:
     • 手工鸡肉粒(幼犬配方) 500g x1
       单价: ¥115.00 (会员价)

     运费: ¥0 (满50包邮 ✓)
     总计: ¥115.00

     请确认收货信息:
     📍 张小姐 138****8000
        北京市朝阳区xxx路xxx号xxx室

     信息正确请回复'确认下单',需要修改请告诉我哦~"

用户: "地址改成公司,朝阳区xx大厦"
AI: "好的,已帮您修改为:
     📍 张小姐 138****8000
        北京市朝阳区xx大厦

     确认无误请回复'确认下单'~"

用户: "确认下单"
AI: [调用 /place_order skill]
    "✅ 订单创建成功!

     订单号: PS260110A1B2C3
     商品: 手工鸡肉粒(幼犬配方) 500g x1
     应付金额: ¥115.00

     [显示微信收款码图片]

     请扫码支付后,将支付截图发给我,我会马上为您确认订单~
     或者您也可以直接微信转账到: xxxxx

     ⏰ 订单会保留2小时,超时自动取消哦"

=== 支付确认阶段 ===

用户: [上传支付截图]
AI: [人工审核,确认支付]
    "收到您的付款啦!感谢信任❤️

     订单状态已更新:
     ✅ 支付成功
     📦 准备发货中

     我们会在24小时内为您发货,到时会告诉您快递单号~

     💝 温馨提示:
     - 收到货后先少量试吃,观察宝宝适应情况
     - 密封保存,开封后7天内吃完
     - 如有任何问题随时找我哦!"

=== 物流跟踪阶段 ===

[第二天]
AI(主动推送): "主人,您的订单已发货啦!

     🚚 物流信息:
     快递公司: 顺丰速运
     快递单号: SF1234567890

     [查看物流详情]按钮

     预计明天送达,请保持手机畅通哦~"

用户: "我的快递到哪了?"
AI: [调用 query_order skill + 物流查询API]
    "主人,您的包裹正在路上:

     📦 订单号: PS260110A1B2C3
     🚚 快递单号: SF1234567890

     最新物流:
     ✓ 2026-01-11 08:30 快件已到达北京转运中心
     ✓ 2026-01-10 20:15 快件已从上海发出
     ✓ 2026-01-10 18:00 商家已发货

     预计今天下午送达,快递员会提前联系您的~"

=== 售后服务阶段 ===

[收货后]
用户: "收到了,谢谢!"
AI: "太好了!请问您家宝宝吃得怎么样呀?喜欢吗?😊"

用户: "超级喜欢!一给就扑上来了哈哈"
AI: "哇!看来选对了呢!泰迪宝宝开心,妈妈也开心❤️

     💡 小贴士:
     - 500g记得密封保存哦
     - 可以用来做训练奖励,效果特别好
     - 快吃完的时候告诉我,我给您安排复购优惠~

     如果觉得不错,欢迎分享给其他狗狗家长哦!"

[一周后]
用户: "我家狗狗吃了之后便便有点软,正常吗?"
AI: [从知识库检索相关知识]
    "主人别担心,幼犬换食物或新食物会有短暂的适应期。

     建议:
     1. 先暂停2天零食,观察便便是否恢复
     2. 恢复后从少量开始(每天5粒),逐渐增加
     3. 注意观察是否有其他症状

     如果2天后还是软便,可能是:
     - 对鸡肉轻微不耐受(可以试试牛肉款)
     - 吃得太多(控制在每天10粒以内)

     如果伴随呕吐、食欲不振,建议带去看医生哦!

     有任何问题随时找我~"

[一个月后]
AI(主动): "主人,您上次买的鸡肉粒应该快吃完了吧?
     要不要再来一单?

     💰 复购优惠:
     500g装 原价¥128,复购价¥108 (省¥20!)

     要的话我帮您下单,地址还是上次那个对吧?"

用户: "好啊,还是老地址"
AI: [调用 /place_order skill]
    "已为您生成订单!
     订单号: PS260210B2C3D4
     ..."
```

### 5.2 更多场景

**场景1: 多商品对比**
```
用户: "鸡肉和牛肉哪个更好?"
AI: [检索知识库对比]
    "主人,鸡肉和牛肉各有特点:

     🐔 鸡肉系列:
     ✓ 蛋白质含量: 28%
     ✓ 脂肪含量: 8% (低脂)
     ✓ 适合: 减肥、挑食、肠胃敏感
     ✓ 特点: 易消化,适口性好
     ✓ 价格: ¥25/100g

     🐮 牛肉系列:
     ✓ 蛋白质含量: 32% (更高)
     ✓ 脂肪含量: 12%
     ✓ 适合: 运动量大、增重、发育期
     ✓ 特点: 补铁补锌,肌肉发育
     ✓ 价格: ¥28/100g

     您家泰迪3个月正在发育,两种都可以!
     如果偏瘦建议牛肉,如果偏胖建议鸡肉~
     要不要两种都试试,换着吃营养更全面?"
```

**场景2: 过敏咨询**
```
用户: "我家狗狗吃了上次的鸡肉后身上起红疹"
AI: "主人,这可能是鸡肉过敏反应😟

     建议:
     1. ⚠️ 立即停止喂食鸡肉类零食
     2. 观察红疹是否消退
     3. 如果严重,请及时就医

     替代方案:
     🐟 三文鱼系列 - 低敏配方,富含Omega-3
     🐑 羊肉系列 - 低敏肉类,补充营养

     这两款都是低敏食材,过敏犬的首选!

     您可以申请退换货,我们支持:
     ✓ 7天无理由退货
     ✓ 过敏包换

     要我帮您处理吗?"
```

**场景3: 批量采购**
```
用户: "我想给狗狗囤点零食,有没有套餐?"
AI: "有的!我给您推荐几个超值套餐:

     🎁 套餐一: 新手尝鲜装
     - 鸡肉粒100g + 牛肉粒100g + 三文鱼饼干100g
     - 原价: ¥84, 套餐价: ¥68 (省¥16)

     🎁 套餐二: 一个月囤货装
     - 鸡肉粒500g + 牛肉粒500g
     - 原价: ¥256, 套餐价: ¥218 (省¥38)

     🎁 套餐三: 豪华全家桶
     - 5种口味各500g (鸡/牛/鱼/羊/混合)
     - 原价: ¥640, 套餐价: ¥499 (省¥141!)

     💝 买套餐还送:
     - 密封保鲜罐 x1
     - 宠物湿巾 x1包

     您家泰迪建议套餐二,够吃2个月,性价比最高!"
```

---

## 六、技术实现要点

### 6.1 商品知识库自动化

**商品信息录入工具:**

```go
// pkg/tools/product_importer.go

type ProductImporter struct {
    core *core.Core
}

// 从CSV/Excel导入商品,自动生成Knowledge
func (p *ProductImporter) ImportFromCSV(ctx context.Context, spaceID, csvPath string) error {
    // 1. 读取CSV
    products := p.parseCSV(csvPath)

    // 2. 为每个商品生成Markdown内容
    for _, product := range products {
        markdown := p.generateProductMarkdown(product)

        // 3. 创建Knowledge
        knowledge := &types.Knowledge{
            SpaceID:     spaceID,
            Resource:    "product",
            Title:       product.Name,
            Tags:        product.Tags,
            Content:     markdown,
            ContentType: types.KNOWLEDGE_CONTENT_TYPE_MARKDOWN,
        }

        p.core.Store().KnowledgeStore().Create(ctx, knowledge)

        // 4. 创建库存记录
        inventory := &Inventory{
            SKU:             product.SKU,
            KnowledgeID:     knowledge.ID,
            Stock:           product.InitialStock,
            AvailableStock:  product.InitialStock,
        }
        p.core.Store().InventoryStore().Create(ctx, inventory)
    }
}

// 生成商品Markdown
func (p *ProductImporter) generateProductMarkdown(product *Product) string {
    return fmt.Sprintf(`
# %s

## 商品编码
SKU: %s

## 商品简介
%s

## 主要成分
%s

## 营养成分
%s

## 适用范围
- 年龄: %s
- 犬种: %s
- 体型: %s

## 产品特点
%s

## 规格与价格
%s

## 库存状态
%s

## 保质期与保存
%s

## 客户评价精选
%s

## 常见问题
%s
    `, product.Name, product.SKU, product.Description, ...)
}
```

### 6.2 价格与库存实时性

**方案: 知识库定期同步 + 工具实时查询**

1. **知识库内容**: 包含价格和库存范围("充足"/"紧张"/"缺货")
2. **下单时**: Skill调用时查询精确库存和最新价格
3. **定时同步**: 每小时更新知识库中的库存状态描述

```go
// 定时任务: 同步库存状态到知识库
func SyncInventoryToKnowledge(ctx context.Context) {
    inventories, _ := store.InventoryStore().GetAll(ctx)

    for _, inv := range inventories {
        status := "库存充足"
        if inv.AvailableStock < 10 {
            status = "⚠️ 库存紧张,仅剩" + inv.AvailableStock + "份"
        } else if inv.AvailableStock == 0 {
            status = "❌ 暂时缺货"
        }

        // 更新Knowledge中的库存状态段落
        knowledge, _ := store.KnowledgeStore().Get(ctx, inv.KnowledgeID)
        updatedContent := updateInventorySection(knowledge.Content, status)
        store.KnowledgeStore().Update(ctx, knowledge.ID, updatedContent)
    }
}
```

### 6.3 对话上下文管理

**场景: 用户说"来一包"时,AI如何知道是哪个商品?**

**方案: 利用现有ChatSession机制**

```go
// 在ChatSession的Pin功能中存储当前推荐的商品
type ShoppingContext struct {
    CurrentRecommendation []Product  // 当前推荐的商品
    Cart                  []CartItem // 购物车
    LastMentionedProduct  *Product   // 最近提到的商品
}

// 当AI推荐商品后,更新Session Pin
func UpdateShoppingContext(sessionID string, products []Product) {
    ctx := ShoppingContext{
        CurrentRecommendation: products,
        LastMentionedProduct:  &products[0],
    }

    // 存储到ChatSessionPin
    pin := &types.ChatSessionPin{
        SessionID: sessionID,
        Content:   json.Marshal(ctx),
        Version:   2,
    }
    store.ChatSessionPinStore().Upsert(ctx, pin)
}

// 当用户说"来一包"时,AI从Pin中获取上下文
func (s *PlaceOrderSkill) Execute(ctx context.Context, args map[string]interface{}) {
    // 如果用户没有明确指定SKU,从Session Pin获取
    if args["sku"] == "" {
        pin, _ := store.ChatSessionPinStore().Get(ctx, sessionID)
        shoppingCtx := &ShoppingContext{}
        json.Unmarshal(pin.Content, shoppingCtx)

        // 使用最近推荐的商品
        args["sku"] = shoppingCtx.LastMentionedProduct.SKU
    }

    // ... 继续下单流程
}
```

### 6.4 多轮对话优化

**示例: AI主动确认**

```
用户: "来一包鸡肉粒"
AI: "好的!请问您要:
     1. 100g装 ¥25
     2. 500g装 ¥115 (更划算)
     3. 1000g装 ¥215

     回复数字选择,或直接说'100g'、'500g'~"

用户: "2"
AI: "收到!500g装 x1 已加入购物车~
     还需要其他的吗?不需要的话直接回复'下单'就可以了"
```

**实现:**
- AI在推荐后返回"待确认"状态
- 记录在Session Pin中
- 下一轮对话时识别用户的选择(数字/规格名)

---

## 七、优势总结

### 7.1 对比传统电商

| 维度 | 传统电商 | 对话式电商(QukaAI) |
|------|---------|-------------------|
| **交互方式** | 点击、浏览、筛选 | 自然语言对话 |
| **商品发现** | 搜索框、分类导航 | AI智能推荐 |
| **决策支持** | 用户自行对比评价 | AI专业解答+知识库 |
| **购物流程** | 多步骤页面跳转 | 对话中无缝下单 |
| **售后服务** | 提交工单/人工客服 | AI即时响应 |
| **情感连接** | ❌ 冷冰冰 | ✅ 有温度 |
| **学习成本** | 需要熟悉界面 | 说话就行 |
| **复购率** | 靠促销 | 靠关系维护 |

### 7.2 核心竞争力

1. **专业性**: 每个用户都有专属宠物营养师
2. **便捷性**: 微信群聊天就能买,无需下载APP
3. **精准性**: 基于RAG的智能推荐,比传统算法更懂用户
4. **信任度**: 持续的对话建立信任,提升复购
5. **差异化**: 同质化市场中的独特体验

### 7.3 适用场景

**最适合的场景:**
- ✅ 私域电商(微信社群、朋友圈)
- ✅ 专业性强的产品(需要咨询指导)
- ✅ 高复购率产品(建立长期关系)
- ✅ 客单价中等(¥50-500)
- ✅ SKU数量适中(10-100个)

**宠物零食完美匹配:**
- 用户需要专业建议(犬种、年龄、健康)
- 高复购率(每月都要买)
- 私域流量为主
- 商品数量适中

---

## 八、实施路线图

### Phase 1: MVP (2周)

**目标**: 验证对话式购物可行性

**功能:**
- ✅ 手动录入5-10个商品到Knowledge
- ✅ 实现ProductRecommendationTool
- ✅ 实现place_order Skill
- ✅ 实现query_order Skill
- ✅ 简单的订单表(ecommerce_order)
- ✅ 简单的库存表(ecommerce_inventory)
- ✅ 手动支付确认流程

**测试场景:**
```
完整流程测试:
1. 用户咨询 → AI推荐
2. 用户下单 → AI调用Skill创建订单
3. 用户支付 → 商家手动确认
4. 用户查询 → AI返回订单状态
```

### Phase 2: 增强 (1周)

**功能:**
- ✅ 批量导入工具(CSV → Knowledge)
- ✅ 库存同步定时任务
- ✅ 物流API集成
- ✅ 完善Prompt(专业性、亲和力)
- ✅ 多轮对话优化
- ✅ 添加购物车Skill

### Phase 3: 完善 (1-2周)

**功能:**
- ✅ 支付集成(微信支付/虎皮椒)
- ✅ 售后系统(退换货)
- ✅ 用户评价收集
- ✅ 复购提醒
- ✅ 数据统计(销量、热门商品)

### Phase 4: 扩展 (持续)

**功能:**
- ✅ 会员积分
- ✅ 优惠券
- ✅ 宠物健康档案
- ✅ 订阅制(定期送货)
- ✅ 多店铺支持

---

## 九、风险与挑战

### 9.1 技术挑战

| 挑战 | 应对方案 |
|------|---------|
| AI理解错误(如用户说"来一包",不知道哪个) | 多轮确认 + Session上下文管理 |
| RAG检索不准确 | 优化Knowledge内容结构 + 关键字增强 |
| 库存超卖 | 下单时实时校验 + 锁定机制 |
| Skill执行失败 | 异常捕获 + 友好错误提示 |

### 9.2 用户体验挑战

| 挑战 | 应对方案 |
|------|---------|
| 用户不习惯对话购物 | 引导话术 + 示例对话 |
| 对话太长影响效率 | 提供快捷指令(如"直接下单上次那款") |
| 无法直观浏览所有商品 | AI主动推送"今日推荐"/"新品上市" |

### 9.3 商业挑战

| 挑战 | 应对方案 |
|------|---------|
| 说服商家接受新模式 | 小范围试点 + 数据证明 |
| 初期商品录入工作量 | 提供批量导入工具 |
| 用户转化率未知 | A/B测试对比传统方式 |

---

## 十、成功指标

### 10.1 核心指标

**用户体验指标:**
- 对话转化率: 咨询 → 下单 > 30%
- 平均对话轮次: < 10轮完成下单
- 用户满意度: > 4.5/5

**商业指标:**
- 客单价: ¥80-150
- 复购率: > 60% (3个月内)
- 退货率: < 5%

**效率指标:**
- AI响应时间: < 2秒
- 下单流程时长: < 3分钟
- 人工介入率: < 20%

---

## 十一、总结

### 核心创新点

1. **商品即知识**: 将商品融入Knowledge体系,利用RAG强大的检索能力
2. **对话即购物**: 打破传统电商界面,回归人与人的交流本质
3. **AI即导购**: 每个用户都有专业、耐心、24小时在线的顾问
4. **系统保持纯粹**: QukaAI依然是RAG系统,只是知识库的应用场景更丰富

### 关键成功因素

1. **知识库质量**: 商品描述要专业、详细、有温度
2. **AI话术**: Prompt要专业、亲切、像朋友
3. **流程顺畅**: Skill设计要简洁、容错
4. **持续优化**: 根据对话数据不断改进

### 长期愿景

这不仅仅是给宠物零食工作室做一个销售系统,而是在探索**对话式商务(Conversational Commerce)**的新模式:

- 未来所有需要专业建议的商品都可以这样卖
- 美妆、保健品、母婴用品、咖啡豆...
- QukaAI成为"对话式商务平台"
- 每个商家都是一个Space,都有自己的AI导购

**这才是真正的创新!** 🚀

---

**文档版本**: v2.0 (对话式电商)
**创建时间**: 2026-01-10
**作者**: Claude Code
**状态**: 待用户确认
