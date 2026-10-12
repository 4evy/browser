// Collect dated store evidence and inspect packages without executing them
package main

import (
	"archive/zip"
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/4evy/tweezle/addons/foundation/chrome/extensions"
	fileutil "github.com/4evy/tweezle/internal/file"
	repojson "github.com/4evy/tweezle/internal/json"
)

const store = "https://chromewebstore.google.com"

var categories = []string{
	"make_chrome_yours/accessibility", "make_chrome_yours/functionality",
	"make_chrome_yours/privacy", "productivity/workflow", "productivity/developer",
	"productivity/education", "productivity/communication", "productivity/tools",
	"lifestyle/art", "lifestyle/entertainment", "lifestyle/games",
	"lifestyle/household", "lifestyle/fun", "lifestyle/news", "lifestyle/shopping",
	"lifestyle/social", "lifestyle/travel", "lifestyle/well_being",
}

// Popular extensions absent from category discovery, verified on their listings
var supplementalIDs = []string{
	"dhdgffkkebhmkfjojejmpbldmpobfkfo", // Tampermonkey
	"hdokiejnpimakedhajhdlcegeplioahd", // LastPass
	"bmnlcjabgnpnenekpadlanbbkooimhnj", // Honey
	"nngceckbapebfimnlniiiahkandclblb", // Bitwarden
	"cjpalhdlnbpafiamejdnhcphjbkeiagm", // uBlock Origin
	"dbepggeogbaibhgnhhndojpepiihcmeb", // Vimium
	"jinjaccalgkegednnccohejagnlnfdag", // Violentmonkey
	"fhbjgbiflinjbdggehcddcbncdddomop", // Postman
	"fmkadmapgofadopljbjfkapdkoienihi", // React Developer Tools
	"jnihajbhpnppcggbcgedagnkighmdlei", // EditThisCookie
	"bfbameneiokkgbdmiekhjnmfkcnldhhm", // Web Developer
	"eimadpbcbfnmbkopoojfekhnkhdbieeh", // Dark Reader
	"gcbommkclmclpchllfjekcdonpmejbdp", // HTTPS Everywhere
	"pkehgijcmpdhfbdbbnkijodmdjhbjlgp", // Privacy Badger
	"clngdbkpkpeebahjckkjfobafhncgmne", // Stylus
	"mnjggcdmjocbbbhaepdhchncahnbgone", // SponsorBlock
	"chphlpgkkbolifaimnlloiipkdnihall", // OneTab
	"aapbdbdomjkkjkaonfhkkikfgjllcleb", // Google Translate
	"kbfnbcaeplbcioakkpcpgfkobkghlhen", // Grammarly
}

var (
	idPattern     = regexp.MustCompile(`\b[a-p]{32}\b`)
	titlePattern  = regexp.MustCompile(`<meta property="og:title" content="([^"]+)"`)
	usersPattern  = regexp.MustCompile(`([\d,]{3,})\+?\s*users`)
	githubPattern = regexp.MustCompile(`https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+`)
	markers       = map[string]*regexp.Regexp{
		"local":            regexp.MustCompile(`(?i)(?:chrome|browser)\.storage\.local`),
		"sync":             regexp.MustCompile(`(?i)(?:chrome|browser)\.storage\.sync`),
		"managed":          regexp.MustCompile(`(?i)(?:chrome|browser)\.storage\.managed`),
		"session":          regexp.MustCompile(`(?i)(?:chrome|browser)\.storage\.session`),
		"indexeddb":        regexp.MustCompile(`(?i)\bindexedDB\b`),
		"localstorage":     regexp.MustCompile(`(?i)\blocalStorage\b`),
		"compression":      regexp.MustCompile(`(?i)LZString|compressToEncodedURIComponent|pako|fflate`),
		"migration":        regexp.MustCompile(`(?i)\bmigrat(?:e|ion|ions|ing)\b`),
		"dynamic_rules":    regexp.MustCompile(`(?i)updateDynamicRules|updateSessionRules`),
		"native_messaging": regexp.MustCompile(`(?i)connectNative|sendNativeMessage`),
	}
)

type record = map[string]any

type researcher struct {
	root, cache string
	client      extensions.HTTPClient
}

func readJSON(file string, value any) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if err := repojson.Default.DecodeInto(bytes.NewReader(data), value); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	return nil
}

func writeJSON(file string, value any) error {
	_, err := fileutil.WriteJSONIfChanged(file, value, fileutil.DefaultFilePerm)
	return err
}

func hash(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (r *researcher) fetch(address, file string) ([]byte, error) {
	compressed, err := os.ReadFile(file + ".gz")
	if err == nil && len(compressed) > 0 {
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return nil, err
		}
		defer func() { _ = reader.Close() }()
		return io.ReadAll(reader)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	data, err := os.ReadFile(file)
	if err == nil && len(data) > 0 {
		return data, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	data, err = r.client.Read(context.Background(), address)
	if err != nil {
		return nil, err
	}
	if filepath.Ext(file) == ".html" {
		err = fileutil.WriteWith(file+".gz", fileutil.DefaultFilePerm, func(output io.Writer) error {
			writer := gzip.NewWriter(output)
			_, writeErr := writer.Write(data)
			return errors.Join(writeErr, writer.Close())
		})
	} else {
		err = fileutil.WriteFile(file, data, fileutil.DefaultFilePerm)
	}
	return data, err
}

// Preserve input order while limiting concurrent network and archive work
func pool[T, R any](items []T, fn func(T) R) []R {
	results := make([]R, len(items))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range 6 {
		workers.Go(func() {
			for index := range jobs {
				results[index] = fn(items[index])
			}
		})
	}
	for index := range items {
		jobs <- index
	}
	close(jobs)
	workers.Wait()
	return results
}

func (r *researcher) metadata(id string) record {
	address := store + "/detail/" + id + "?hl=en"
	page, err := r.fetch(address, filepath.Join(r.cache, "listings", id+".html"))
	if err != nil {
		return record{"id": id, "error": err.Error()}
	}
	title := titlePattern.FindSubmatch(page)
	users := usersPattern.FindSubmatch(page)
	if title == nil || users == nil {
		return record{"id": id, "error": "missing title or user count"}
	}
	count, err := strconv.Atoi(strings.ReplaceAll(string(users[1]), ",", ""))
	if err != nil {
		return record{"id": id, "error": err.Error()}
	}
	repos := map[string]bool{}
	for _, repo := range githubPattern.FindAllString(string(page), -1) {
		repos[repo] = true
	}
	return record{
		"id": id, "name": strings.TrimSuffix(html.UnescapeString(string(title[1])), " - Chrome Web Store"),
		"users": count, "listing_url": address, "listing_sha256": hash(page),
		"github_links": slices.Sorted(maps.Keys(repos)),
	}
}

func (r *researcher) collect() error {
	candidates := map[string]bool{}
	for _, id := range supplementalIDs {
		candidates[id] = true
	}
	categoryCounts := map[string]int{}
	for _, category := range categories {
		data, err := r.fetch(store+"/category/extensions/"+category+"?hl=en",
			filepath.Join(r.cache, "categories", strings.ReplaceAll(category, "/", "-")+".html"))
		if err != nil {
			return err
		}
		ids := map[string]bool{}
		for _, id := range idPattern.FindAllString(string(data), -1) {
			ids[id], candidates[id] = true, true
		}
		if len(ids) == 0 {
			return fmt.Errorf("no IDs in category %s", category)
		}
		categoryCounts[category] = len(ids)
	}
	var seed struct {
		Extensions []struct {
			ID string `json:"id"`
		} `json:"extensions"`
	}
	if err := readJSON(filepath.Join(r.cache, "upstream-top-600.json"), &seed); err == nil {
		for _, item := range seed.Extensions {
			candidates[item.ID] = true
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	records := pool(slices.Sorted(maps.Keys(candidates)), r.metadata)
	valid, failed := []record{}, []record{}
	for _, item := range records {
		if _, exists := item["error"]; exists {
			failed = append(failed, item)
		} else {
			valid = append(valid, item)
		}
	}
	slices.SortFunc(valid, func(a, b record) int {
		return cmp.Or(
			cmp.Compare(b["users"].(int), a["users"].(int)),
			cmp.Compare(a["id"].(string), b["id"].(string)),
		)
	})
	if len(valid) < 150 {
		return fmt.Errorf("only %d listings have user counts", len(valid))
	}
	for index, item := range valid[:150] {
		item["rank"] = index + 1
	}
	listingURLs := make([]string, len(supplementalIDs))
	for index, id := range supplementalIDs {
		listingURLs[index] = store + "/detail/" + id + "?hl=en"
	}
	output := record{
		"collected_at": time.Now().UTC().Format(time.RFC3339Nano),
		"method":       "150 highest displayed user counts among category-listing IDs, the July 2026 top-600 seed, and supplemental popular IDs; ties sorted by ID",
		"limitations": []string{
			"Category discovery cannot prove a global top 150",
			"Store user counts are rounded displayed counts, not exact installs",
			"Popularity does not establish configuration complexity",
		},
		"seed_url":   "https://github.com/HughParry/chrome-extension-identifier/blob/main/data/top-600.json",
		"categories": categoryCounts, "supplemental_listing_urls": listingURLs,
		"candidate_count": len(candidates), "valid_listing_count": len(valid),
		"failed_listings": failed, "extensions": valid[:150],
	}
	if err := writeJSON(filepath.Join(r.cache, "pool.json"), output); err != nil {
		return err
	}
	fmt.Printf("Collected %d listings from %d candidates\n", len(valid), len(candidates))
	return nil
}

func crxArchive(data []byte) (*zip.Reader, error) {
	if len(data) < 12 || string(data[:4]) != "Cr24" {
		return nil, fmt.Errorf("response is not a CRX or has a truncated header")
	}
	version := binary.LittleEndian.Uint32(data[4:8])
	var offset uint64
	switch version {
	case 3:
		offset = 12 + uint64(binary.LittleEndian.Uint32(data[8:12]))
	case 2:
		if len(data) < 16 {
			return nil, fmt.Errorf("truncated CRX2 header")
		}
		offset = 16 + uint64(binary.LittleEndian.Uint32(data[8:12])) + uint64(binary.LittleEndian.Uint32(data[12:16]))
	default:
		return nil, fmt.Errorf("unsupported CRX version %d", version)
	}
	if offset > uint64(len(data)) {
		return nil, fmt.Errorf("truncated CRX payload")
	}
	return zip.NewReader(bytes.NewReader(data[offset:]), int64(len(data))-int64(offset))
}

func readEntry(entry *zip.File) ([]byte, error) {
	const limit = 30 * 1024 * 1024
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("%s exceeds 30 MiB inspection limit", entry.Name)
	}
	return data, nil
}

func (r *researcher) packageEvidence(item record) (record, error) {
	id, ok := item["id"].(string)
	if !ok || !extensions.ValidExtensionID(id) {
		return nil, fmt.Errorf("invalid extension ID: %v", item["id"])
	}
	address, err := extensions.ChromeStoreCRXDownloadURLForVersion(
		"https://clients2.google.com/service/update2/crx", id, "150.0.0.0",
	)
	if err != nil {
		return nil, err
	}
	data, err := r.fetch(address, filepath.Join(r.cache, "packages", id+".crx"))
	if err != nil {
		return nil, err
	}
	archive, err := crxArchive(data)
	if err != nil {
		return nil, err
	}
	manifestFile, err := archive.Open("manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest record
	err = repojson.Default.DecodeInto(io.LimitReader(manifestFile, 30*1024*1024), &manifest)
	closeErr := manifestFile.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	destination := filepath.Join(r.cache, "unpacked", id)
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return nil, err
	}
	unpacked, err := os.OpenRoot(destination)
	if err != nil {
		return nil, err
	}
	defer func() { _ = unpacked.Close() }()
	evidence, skipped := []record{}, []record{}
	repos := map[string]bool{}
	if links, ok := item["github_links"].([]any); ok {
		for _, link := range links {
			if repo, ok := link.(string); ok {
				repos[repo] = true
			}
		}
	}
	counts := map[string]int{}
	for key := range markers {
		counts[key] = 0
	}
	var jsBytes uint64
	jsFiles := 0
	for _, entry := range archive.File {
		if path.IsAbs(entry.Name) || slices.Contains(strings.Split(entry.Name, "/"), "..") {
			return nil, fmt.Errorf("unsafe archive path: %s", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			skipped = append(skipped, record{"file": entry.Name, "reason": "archive symlink"})
			if err := unpacked.Remove(entry.Name); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			continue
		}
		if entry.UncompressedSize64 > 30*1024*1024 {
			skipped = append(skipped, record{"file": entry.Name, "reason": "larger than 30 MiB inspection limit"})
			continue
		}
		ext := path.Ext(entry.Name)
		if !slices.Contains([]string{".js", ".mjs", ".json", ".html"}, ext) {
			continue
		}
		body, err := readEntry(entry)
		if err != nil {
			return nil, err
		}
		text := strings.ToValidUTF8(string(body), "\uFFFD")
		if err := unpacked.MkdirAll(path.Dir(entry.Name), 0o755); err != nil {
			return nil, err
		}
		if err := unpacked.WriteFile(entry.Name, []byte(text), 0o644); err != nil {
			return nil, err
		}
		if ext == ".js" || ext == ".mjs" {
			jsBytes += entry.UncompressedSize64
			jsFiles++
			hits := map[string]int{}
			total := 0
			for key, pattern := range markers {
				hits[key] = len(pattern.FindAllStringIndex(text, -1))
				counts[key] += hits[key]
				total += hits[key]
			}
			if total > 0 {
				evidence = append(evidence, record{"file": entry.Name, "markers": hits, "sha256": hash(body)})
			}
		}
		for _, repo := range githubPattern.FindAllString(text, -1) {
			repos[repo] = true
		}
	}
	options := manifest["options_page"]
	if options == nil || options == "" {
		options = manifest["options_ui"]
	}
	var storageSchema any
	if storage, ok := manifest["storage"].(map[string]any); ok {
		storageSchema = storage["managed_schema"]
	}
	permissions, exists := manifest["permissions"]
	if !exists {
		permissions = []any{}
	}
	hostPermissions, exists := manifest["host_permissions"]
	if !exists {
		hostPermissions = []any{}
	}
	return record{
		"status": "inspected", "package_url": address, "package_sha256": hash(data),
		"version": manifest["version"], "manifest_version": manifest["manifest_version"],
		"permissions": permissions, "host_permissions": hostPermissions,
		"options_page": options, "storage_schema": storageSchema,
		"js_file_count": jsFiles, "js_bytes": jsBytes, "markers": counts,
		"evidence": evidence, "skipped_files": skipped,
		"repository_candidates": slices.Sorted(maps.Keys(repos)), "configuration_review": "pending",
	}, nil
}

type inspection struct {
	result record
	err    error
}

func (r *researcher) inspectPackage(item record) inspection {
	id, ok := item["id"].(string)
	if !ok || !extensions.ValidExtensionID(id) {
		return inspection{err: fmt.Errorf("invalid extension ID: %v", item["id"])}
	}
	saved := filepath.Join(r.cache, "inspections", id+".json")
	var result record
	if err := readJSON(saved, &result); err == nil {
		if result["status"] == "inspected" {
			return inspection{result: result}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return inspection{err: err}
	}
	result, err := r.packageEvidence(item)
	if err != nil {
		result = record{"status": "unavailable", "error": err.Error()}
	}
	if err := writeJSON(saved, result); err != nil {
		return inspection{err: err}
	}
	fmt.Printf("%3v %v: %s\n", item["rank"], item["name"], result["status"])
	return inspection{result: result}
}

func (r *researcher) inspect() error {
	var output record
	file := filepath.Join(r.cache, "pool.json")
	if err := readJSON(file, &output); err != nil {
		return err
	}
	list, ok := output["extensions"].([]any)
	if !ok {
		return fmt.Errorf("pool.json: missing extensions array")
	}
	items := make([]record, len(list))
	for index, item := range list {
		var ok bool
		items[index], ok = item.(map[string]any)
		if !ok {
			return fmt.Errorf("pool.json: extension %d is not an object", index)
		}
	}
	if err := os.MkdirAll(filepath.Join(r.cache, "inspections"), 0o755); err != nil {
		return err
	}
	for index, result := range pool(items, r.inspectPackage) {
		if result.err != nil {
			return result.err
		}
		items[index]["inspection"] = result.result
	}
	return writeJSON(file, output)
}

func run() error {
	_, source, _, _ := runtime.Caller(0)
	root := flag.String("root", filepath.Dir(source), "directory containing corpus.json")
	flag.Usage = func() {
		_, _ = fmt.Fprintln(flag.CommandLine.Output(), "Usage: corpus [-root DIRECTORY] collect|inspect|shortlist")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 || !slices.Contains([]string{"collect", "inspect", "shortlist"}, flag.Arg(0)) {
		flag.Usage()
		return fmt.Errorf("choose collect, inspect, or shortlist")
	}
	cache := os.Getenv("EXTENSION_RESEARCH_CACHE")
	if cache == "" {
		cache = filepath.Join(*root, "../../../.sources/extension-research")
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return err
	}
	r := researcher{root: *root, cache: cache, client: extensions.HTTPClient{
		UserAgent:    "tweezle-extension-config-research/1.0",
		Headers:      map[string]string{"Accept-Language": "en-US,en;q=0.9"},
		RetryWaitMin: time.Second,
		RetryWaitMax: 4 * time.Second,
	}}
	if flag.Arg(0) == "collect" {
		return r.collect()
	}
	if flag.Arg(0) == "inspect" {
		return r.inspect()
	}
	return r.shortlist()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
