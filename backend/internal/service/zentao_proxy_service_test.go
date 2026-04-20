package service

import "testing"

func TestResolveBranchesFromProducts(t *testing.T) {
	products := []ZentaoProduct{
		{ID: 100, Name: "智学平台", Type: "normal", Line: 10, Program: 88},
		{ID: 101, Name: "智学平台-A", Type: "branch", Line: 10, Program: 88},
		{ID: 102, Name: "智学平台-B", Type: "branch", Line: 10, Program: 100},
		{ID: 103, Name: "其他产品", Type: "branch", Line: 11, Program: 89},
	}

	branches := resolveBranchesFromProducts("100", products)
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(branches))
	}
	if branches[0].ID != 101 || branches[1].ID != 102 {
		t.Fatalf("unexpected branch ids: %+v", branches)
	}
}

func TestParseBranchesFromHTML(t *testing.T) {
	body := []byte(`<a href='/x' data-id='1'>ignore</a><a class='branch' href='/product' id='branch-11'>batchChangeBranch-11-foo">智学平台-A</a><a class='branch' href='/product' id='branch-12'>batchChangeBranch-12-bar">智学平台-B</a>`)

	branches := parseBranchesFromHTML(body)
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(branches))
	}
	if branches[0].Name != "智学平台-A" || branches[1].Name != "智学平台-B" {
		t.Fatalf("unexpected branches: %+v", branches)
	}
}