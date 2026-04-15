package service

import "testing"

func TestParseMarkdownTestCases_ParsesStepExpectationTable(t *testing.T) {
	doc := `# 用例标题：Create Corporate Group with Member Offer

## 前置条件

1. 已登录系统
2. 已准备测试数据

## 用例步骤

| 编号 | 步骤 | 预期 |
| -- | --- | --- |
| 1 | 打开页面 | 页面打开成功 |
| 2 | 点击提交 | 提交成功 |
`

	cases := parseMarkdownTestCases(doc)
	if len(cases) != 1 {
		t.Fatalf("expected 1 parsed case, got %d", len(cases))
	}

	if cases[0].Precondition != "1. 已登录系统\n2. 已准备测试数据" {
		t.Fatalf("unexpected precondition: %q", cases[0].Precondition)
	}
	if cases[0].Steps != "打开页面\n点击提交" {
		t.Fatalf("unexpected steps: %q", cases[0].Steps)
	}
	if cases[0].Expected != "页面打开成功\n提交成功" {
		t.Fatalf("unexpected expected: %q", cases[0].Expected)
	}
}

func TestNormalizeGeneratedTestCases_BackfillsExpectedFromMarkdownDoc(t *testing.T) {
	output := &GenTestOutput{
		TestCases: []GenTestCase{
			{
				Title:        "Create Corporate Group with Member Offer and Verify Group Offering",
				Precondition: "1. 已登录系统",
				Steps:        "1. 进入 Business Hall\n2. 点击 Submit",
				Expected:     "提交后看到正确结果",
			},
		},
		TestDoc: GenTestDoc{
			Content: `# 用例标题：Create Corporate Group with Member Offer

## 前置条件

1. 已登录系统

## 用例步骤

| 编号 | 步骤 | 预期 |
| -- | --- | --- |
| 1 | 进入 Business Hall | 成功进入页面 |
| 2 | 点击 Submit | 提交成功 |
`,
		},
	}

	normalizeGeneratedTestCases(output)

	if got := output.TestCases[0].Steps; got != "进入 Business Hall\n点击 Submit" {
		t.Fatalf("expected normalized steps, got %q", got)
	}
	if got := output.TestCases[0].Expected; got != "成功进入页面\n提交成功" {
		t.Fatalf("expected normalized expected lines, got %q", got)
	}
}

func TestNormalizeGeneratedTestCases_MatchesByIndexWhenTitlesDiffer(t *testing.T) {
	output := &GenTestOutput{
		TestCases: []GenTestCase{
			{
				Title:    "Alpha Summary",
				Steps:    "1. alpha",
				Expected: "alpha summary",
			},
			{
				Title:    "Beta Summary",
				Steps:    "1. beta",
				Expected: "beta summary",
			},
		},
		TestDoc: GenTestDoc{
			Content: `# 用例标题：Case A

## 用例步骤

| 编号 | 步骤 | 预期 |
| -- | --- | --- |
| 1 | alpha step 1 | alpha expected 1 |
| 2 | alpha step 2 | alpha expected 2 |

# 用例标题：Case B

## 用例步骤

| 编号 | 步骤 | 预期 |
| -- | --- | --- |
| 1 | beta step 1 | beta expected 1 |
`,
		},
	}

	normalizeGeneratedTestCases(output)

	if got := output.TestCases[0].Expected; got != "alpha expected 1\nalpha expected 2" {
		t.Fatalf("expected first case to be normalized by index, got %q", got)
	}
	if got := output.TestCases[1].Expected; got != "beta expected 1" {
		t.Fatalf("expected second case to be normalized by index, got %q", got)
	}
}

func TestNormalizeGeneratedTestCases_ParsesFrontMatterAndEnglishTableHeaders(t *testing.T) {
	output := &GenTestOutput{
		TestCases: []GenTestCase{
			{
				Title:        "验证Group Offering页签不回显群成员Offer和账单介质Offer",
				Precondition: "已登录到CRM系统；系统中存在可用的集团客户数据",
				Steps:        "1. old step\n2. old step",
				Expected:     "only one expected",
			},
		},
		TestDoc: GenTestDoc{
			Content: `---
title: 创建集团群组并验证Group Offering页签不回显群成员Offer和账单介质Offer
type: test
---

# 创建集团群组并验证Group Offering页签不回显群成员Offer和账单介质Offer

## 前置条件

1. 已登录到CRM系统
2. 系统中存在可用的集团客户数据

## 测试步骤

| Step | Procedure | Expected result |
|------|-----------|-----------------|
| 1 | 进入 Business Hall > Create Corporate Group | Create Corporate Group 页面正常显示 |
| 2 | 查询集团客户，点击进入 Create Corp Group 页面 | Create Corp Group 页面加载完成 |
| 3 | 检查 Group Offering 页签下的Offer列表 | 列表中不包含群成员Offer和账单介质Offer |
`,
		},
	}

	normalizeGeneratedTestCases(output)

	if got := output.TestCases[0].Steps; got != "进入 Business Hall > Create Corporate Group\n查询集团客户，点击进入 Create Corp Group 页面\n检查 Group Offering 页签下的Offer列表" {
		t.Fatalf("expected normalized english-header steps, got %q", got)
	}
	if got := output.TestCases[0].Expected; got != "Create Corporate Group 页面正常显示\nCreate Corp Group 页面加载完成\n列表中不包含群成员Offer和账单介质Offer" {
		t.Fatalf("expected normalized english-header expected, got %q", got)
	}
}

func TestNormalizeGeneratedTestCases_DoesNotDependOnFixedTableHeaders(t *testing.T) {
	output := &GenTestOutput{
		TestCases: []GenTestCase{
			{
				Title:    "任意表头用例",
				Steps:    "旧步骤",
				Expected: "旧预期",
			},
		},
		TestDoc: GenTestDoc{
			Content: `# 任意表头用例

## 测试步骤

| No. | Action | Checkpoint |
| --- | ------ | ---------- |
| 1 | 打开页面 | 页面展示成功 |
| 2 | 点击保存 | 保存成功 |
`,
		},
	}

	normalizeGeneratedTestCases(output)

	if got := output.TestCases[0].Steps; got != "打开页面\n点击保存" {
		t.Fatalf("expected generic-header steps, got %q", got)
	}
	if got := output.TestCases[0].Expected; got != "页面展示成功\n保存成功" {
		t.Fatalf("expected generic-header expected, got %q", got)
	}
}
