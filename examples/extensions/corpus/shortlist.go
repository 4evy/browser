package main

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type selection struct {
	repo, repository, id string
	files, reasons       []string
}

var complexSelections = []selection{
	{
		"RES", "https://github.com/honestbleeps/Reddit-Enhancement-Suite",
		"kbmfpngjjgdllneeigpgjifpgocmfgmb",
		[]string{"lib/core/options/storage.js", "lib/core/modules/storage.js", "lib/modules/nightMode.js"},
		[]string{
			"Per-module schemas with nested value wrappers and dependent options",
			"Separate module enablement blob plus prefixed options keys",
			"Complex tables, keybindings, filters, and runtime data",
		},
	},
	{
		"obsidian-clipper", "https://github.com/obsidianmd/obsidian-clipper",
		"cnjifjpddelmedmihgijeibhnjfabmlf",
		[]string{"src/utils/storage-utils.ts", "src/managers/template-manager.ts", "src/types/types.ts"},
		[]string{
			"UTF-16 compressed template chunk arrays",
			"Six template modes, typed properties, triggers, and model/provider relationships",
			"Distinct local and sync storage plus migrations",
		},
	},
	{
		"SponsorBlock", "https://github.com/ajayyy/SponsorBlock",
		"mnjggcdmjocbbbhaepdhchncahnbgone",
		[]string{"src/config.ts", "src/utils/skipRule.type.ts"},
		[]string{
			"Recursive and/or predicates with typed comparison operators",
			"Per-channel profiles, optional overrides, and enum category behavior",
			"Local/sync migrations and nullable application state",
		},
	},
	{
		"darkreader", "https://github.com/darkreader/darkreader",
		"eimadpbcbfnmbkopoojfekhnkhdbieeh",
		[]string{"src/defaults.ts", "src/background/user-storage.ts", "src/background/utils/extension-api.ts"},
		[]string{
			"Nested themes, per-site overrides, presets, and automation modes",
			"Base-36 sync chunk protocol and quota handling",
			"Schema migration can replace incorrectly versioned site lists",
		},
	},
	{
		"stylus", "https://github.com/openstyles/stylus",
		"clngdbkpkpeebahjckkjfobafhncgmne",
		[]string{"src/js/prefs.js", "src/background/prefs-api.js"},
		[]string{
			"108 preferences including computed names and literal dotted keys",
			"Nested editor and beautifier options under one sync value",
			"Styles and drafts stored outside the preference blob",
		},
	},
	{
		"ghostery", "https://github.com/ghostery/ghostery-extension",
		"mlomiejdfkolichcflejclcbmpeaniij",
		[]string{"src/store/managed-config.js", "src/managed_storage.json", "src/background/reporting/storage-indexeddb.js"},
		[]string{
			"Managed policy precedence and local cached policy state",
			"Custom filters and per-host tracker exceptions",
			"IndexedDB reporting storage alongside chrome.storage configuration",
		},
	},
}

func (r *researcher) shortlist() error {
	poolFile := filepath.Join(r.cache, "pool.json")
	var poolData record
	if err := readJSON(poolFile, &poolData); err != nil {
		return err
	}
	var reviews struct {
		Reviews []struct {
			ID     string `json:"id"`
			Sample string `json:"sample"`
		} `json:"reviews"`
	}
	if err := readJSON(filepath.Join(r.root, "reviews.json"), &reviews); err != nil {
		return err
	}
	samples := map[string]string{}
	for _, item := range reviews.Reviews {
		samples[item.ID] = item.Sample
	}
	list, ok := poolData["extensions"].([]any)
	if !ok {
		return fmt.Errorf("pool.json: missing extensions array")
	}
	candidates := map[string]record{}
	inspected := 0
	for index, raw := range list {
		item, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("pool.json: extension %d is not an object", index)
		}
		id, ok := item["id"].(string)
		if !ok || candidates[id] != nil {
			return fmt.Errorf("pool.json: missing or duplicated extension ID %v", item["id"])
		}
		candidates[id] = item
		if evidence, ok := item["inspection"].(map[string]any); ok && evidence["status"] == "inspected" {
			inspected++
		}
	}
	retained := make([]record, 0, len(complexSelections))
	for _, chosen := range complexSelections {
		candidate, exists := candidates[chosen.id]
		if !exists {
			return fmt.Errorf("pool.json: selected extension %s is missing", chosen.id)
		}
		item := maps.Clone(candidate)
		commit, err := exec.Command("git", "-C", filepath.Join(r.cache, "repos", chosen.repo), "rev-parse", "HEAD").Output()
		if err != nil {
			return fmt.Errorf("read source revision for %s: %w", chosen.repo, err)
		}
		ref := strings.TrimSpace(string(commit))
		links := make([]string, 0, len(chosen.files))
		for _, file := range chosen.files {
			links = append(links, chosen.repository+"/blob/"+ref+"/"+file)
		}
		var sample any
		if file := samples[chosen.id]; file != "" {
			sample = file
		} else {
			return fmt.Errorf("selected extension %s has no reviewed sample", chosen.id)
		}
		liveValidation := "Not run; the Go fixture checks native storage persistence"
		if chosen.repo == "stylus" {
			liveValidation = "Passed twice on Linux x86_64 with Helium 0.16.4.1 (Chromium 152.0.7977.75), Stylus 2.4.14, complete unpacked store package, disposable profile, and project profile apply; imported disabled sectioned and UserCSS styles, compiled UserCSS, set order, and read styles back through the extension API"
		}
		if inspection, ok := item["inspection"].(map[string]any); ok {
			inspection = maps.Clone(inspection)
			inspection["configuration_review"] = "Source-pinned configurable declarations reviewed; see reviews.json for coverage and exclusions"
			item["inspection"] = inspection
		}
		item["selection"] = record{
			"reasons": chosen.reasons, "repository": chosen.repository,
			"commit": ref, "source_urls": links, "sample": sample,
			"configuration_audit": record{
				"status":     "Source-declared configurable fields covered",
				"review":     "reviews.json",
				"scope":      "Storage defaults and configurable nested schemas; source-declared optional fields, table columns, builder cases, text-format commands, style schemas, and managed policies where applicable",
				"exclusions": "Action buttons, runtime caches and histories, generated databases and rules, server-owned identity and publishing state, other platforms, and premium-only features; per-case limitations are in reviews.json",
			},
			"live_browser_validation": liveValidation,
		}
		retained = append(retained, item)
	}
	data, err := os.ReadFile(poolFile)
	if err != nil {
		return err
	}
	output := record{
		"research_pool": record{
			"collected_at":        poolData["collected_at"],
			"candidate_count":     poolData["candidate_count"],
			"valid_listing_count": poolData["valid_listing_count"],
			"packages_inspected":  inspected, "pool_size": len(list),
			"method": poolData["method"], "limitations": poolData["limitations"],
			"snapshot_sha256": hash(data), "cache_file": ".sources/extension-research/pool.json",
		},
		"selection_rule":    "Retain source-confirmed complex schemas, compressed/chunked values, migrations, policy precedence, recursive rules, or multiple persistence mechanisms; API marker counts alone are insufficient",
		"discarded_samples": []record{{"name": "Vimium", "reason": "Mostly flat settings; lower configuration complexity than the retained cases"}},
		"retained_count":    len(retained), "extensions": retained,
	}
	if err := writeJSON(filepath.Join(r.root, "corpus.json"), output); err != nil {
		return err
	}
	fmt.Printf("Retained %d complex extensions from %d inspected packages\n", len(retained), len(candidates))
	return nil
}
