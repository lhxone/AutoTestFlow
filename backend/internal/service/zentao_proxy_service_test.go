package service

import "testing"

func TestParseBranchesFromDropMenuHTML(t *testing.T) {
	body := []byte(`<script>productID = 243;</script>
<div class="table-row">
  <div class="table-col col-left">
    <div class='list-group'>
      <a href='/product-browse-243-all--0-story.html' class='selected' data-key='suoyou sy' data-app='product'>所有</a>
      <a href='/product-browse-243-0--0-story.html' class='' data-key='zhugan zg' data-app='product'>主干</a>
      <a href='/product-browse-243-536--0-story.html' class='' data-key='fenzhi-aisaicbe fzasc' data-app='product'>分支-埃塞CBE</a>
      <a href='/execution-browse-243.html' class='' data-app='execution'>忽略我</a>
    </div>
  </div>
</div>`)

	branches := parseBranchesFromDropMenuHTML(body, "243")
	if len(branches) != 3 {
		t.Fatalf("expected 3 branches, got %d", len(branches))
	}
	if branches[0].ID != "all" || branches[0].Name != "所有" {
		t.Fatalf("unexpected first branch: %+v", branches[0])
	}
	if branches[1].ID != "0" || branches[1].Name != "主干" {
		t.Fatalf("unexpected second branch: %+v", branches[1])
	}
	if branches[2].ID != "536" || branches[2].Name != "分支-埃塞CBE" {
		t.Fatalf("unexpected third branch: %+v", branches[2])
	}
}

func TestParseBranchesFromLegacyHTML(t *testing.T) {
	body := []byte(`<a href='/x' data-id='1'>ignore</a><a class='branch' href='/product' id='branch-11'>batchChangeBranch-11-foo">智学平台-A</a><a class='branch' href='/product' id='branch-12'>batchChangeBranch-12-bar">智学平台-B</a>`)

	branches := parseBranchesFromLegacyHTML(body)
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(branches))
	}
	if branches[0].ID != "11" || branches[0].Name != "智学平台-A" {
		t.Fatalf("unexpected first branch: %+v", branches[0])
	}
	if branches[1].ID != "12" || branches[1].Name != "智学平台-B" {
		t.Fatalf("unexpected second branch: %+v", branches[1])
	}
}
